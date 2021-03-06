package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vicanso/pike/pike"

	"github.com/vicanso/pike/cache"
	"github.com/vicanso/pike/util"
)

func TestShouldCompress(t *testing.T) {
	compressTypes := []string{
		"text",
		"javascript",
		"json",
	}
	t.Run("should compress", func(t *testing.T) {
		if !shouldCompress(compressTypes, "json") {
			t.Fatalf("json should be compress")
		}
		if shouldCompress(compressTypes, "image/png") {
			t.Fatalf("image png should not be compress")
		}
	})
}

func TestSave(t *testing.T) {
	client := &cache.Client{
		Path: "/tmp/test.cache",
	}
	err := client.Init()
	if err != nil {
		t.Fatalf("cache init fail, %v", err)
	}
	defer client.Close()
	identity := []byte("save-test")
	t.Run("save no content", func(t *testing.T) {
		resp := &cache.Response{
			TTL:        30,
			StatusCode: http.StatusNoContent,
		}
		save(client, identity, resp, true)
		result, err := client.GetResponse(identity)
		if err != nil || result.TTL != resp.TTL || result.StatusCode != resp.StatusCode {
			t.Fatalf("save no content fail")
		}
	})

	t.Run("save gzip content", func(t *testing.T) {
		data := []byte("data")
		gzipData, _ := util.Gzip(data, 0)
		resp := &cache.Response{
			TTL:               30,
			StatusCode:        http.StatusOK,
			GzipBody:          gzipData,
			CompressMinLength: 1,
		}
		save(client, identity, resp, true)
		result, err := client.GetResponse(identity)
		if err != nil || result.TTL != resp.TTL || result.StatusCode != resp.StatusCode {
			t.Fatalf("save gzip content fail")
		}
		if len(result.Body) != 0 {
			t.Fatalf("raw content should be nil")
		}
		if !bytes.Equal(result.GzipBody, gzipData) {
			t.Fatalf("save gzip content is not equal original data")
		}
		if len(result.BrBody) == 0 {
			t.Fatalf("should save br data")
		}
	})

	t.Run("save br content", func(t *testing.T) {
		data := []byte("data")
		brData, _ := util.BrotliEncode(data, 0)
		resp := &cache.Response{
			TTL:               30,
			StatusCode:        http.StatusOK,
			BrBody:            brData,
			CompressMinLength: 1,
		}
		save(client, identity, resp, true)
		result, err := client.GetResponse(identity)
		if err != nil || result.TTL != resp.TTL || result.StatusCode != resp.StatusCode {
			t.Fatalf("save br content fail")
		}
		if len(result.Body) != 0 {
			t.Fatalf("raw content should be nil")
		}
		if !bytes.Equal(result.BrBody, brData) {
			t.Fatalf("save br content is not equal original data")
		}
		if len(result.GzipBody) == 0 {
			t.Fatalf("should save gzip data")
		}
	})

	t.Run("save content smaller than compress min length", func(t *testing.T) {
		data := []byte("需要一个很大的数据，如果没有，那就设置小的compressMinLength")
		resp := &cache.Response{
			TTL:        30,
			StatusCode: http.StatusOK,
			Body:       data,
		}
		save(client, identity, resp, true)
		result, err := client.GetResponse(identity)
		if err != nil {
			t.Fatalf("save samll content fail, %v", err)
		}
		if len(result.Body) == 0 {
			t.Fatalf("the body of small content response shoul not be nil")
		}
		if len(result.GzipBody) != 0 {
			t.Fatalf("the gzip body of small content response shoul be nil")
		}
	})

	t.Run("save content bigger than compress min length", func(t *testing.T) {
		data := []byte("需要一个很大的数据，如果没有，那就设置小的compressMinLength")
		resp := &cache.Response{
			TTL:               30,
			StatusCode:        http.StatusOK,
			Body:              data,
			CompressMinLength: 1,
		}
		save(client, identity, resp, true)
		result, err := client.GetResponse(identity)
		if err != nil || result.TTL != resp.TTL || result.StatusCode != resp.StatusCode {
			t.Fatalf("save big content fail")
		}
		gzipData := result.GzipBody
		if len(gzipData) == 0 {
			t.Fatalf("big cotent response should be gzip")
		}
		raw, _ := util.Gunzip(gzipData)
		if !bytes.Equal(raw, data) {
			t.Fatalf("big cotent response gzip fail")
		}

		brData := result.BrBody
		if len(brData) == 0 {
			t.Fatalf("big cotent response should be brotli")
		}
		raw, _ = util.BrotliDecode(brData)
		if !bytes.Equal(raw, data) {
			t.Fatalf("big cotent response brotli fail")
		}
	})
}

func TestDispatcher(t *testing.T) {
	client := &cache.Client{
		Path: "/tmp/test.cache",
	}
	err := client.Init()
	if err != nil {
		t.Fatalf("cache init fail, %v", err)
	}
	defer client.Close()
	conf := DispatcherConfig{}
	t.Run("dispatch response", func(t *testing.T) {
		fn := Dispatcher(conf, client)
		req := httptest.NewRequest(http.MethodPost, "/users/me", nil)
		c := pike.NewContext(req)
		c.Identity = []byte("abc")
		c.Status = cache.Fetching
		cr := &cache.Response{
			CreatedAt:  uint32(time.Now().Unix()),
			TTL:        300,
			StatusCode: 200,
			Body:       []byte("ABCD"),
		}
		c.Resp = cr
		err := fn(c, func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("dispatch fail, %v", err)
		}
		if c.Response.Status() != 200 {
			t.Fatalf("the response code should be 200")
		}
		if string(c.Response.Bytes()) != "ABCD" {
			t.Fatalf("the response body should be ABCD")
		}
		// 由于缓存的数据需要写数据库，因此需要延时关闭client
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("dispatch cacheable data", func(t *testing.T) {
		identity := []byte("abc")
		cr := &cache.Response{
			CreatedAt:  uint32(time.Now().Unix()),
			TTL:        300,
			StatusCode: 200,
			Body:       []byte("ABCD"),
		}
		fn := Dispatcher(conf, client)
		req := httptest.NewRequest(http.MethodPost, "/users/me", nil)

		c := pike.NewContext(req)
		c.Identity = identity
		c.Status = cache.Cacheable
		c.Resp = cr
		err := fn(c, func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("dispatch cacheable data fail, %v", err)
		}
		if !bytes.Equal(c.Response.Bytes(), cr.Body) {
			t.Fatalf("dispatch cacheable data fail")
		}
	})

	t.Run("dispatch not modified", func(t *testing.T) {
		identity := []byte("abc")
		cr := &cache.Response{
			CreatedAt:  uint32(time.Now().Unix()),
			TTL:        300,
			StatusCode: 200,
			Body:       []byte("ABCD"),
		}
		fn := Dispatcher(conf, client)
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		c := pike.NewContext(req)
		c.Fresh = true
		c.Identity = identity
		c.Status = cache.Cacheable
		c.Resp = cr
		err := fn(c, func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("dispatch not modified fail, %v", err)
		}
		if c.Response.Status() != http.StatusNotModified {
			t.Fatalf("dispatch not modified fail")
		}
	})

	t.Run("dispatch compress response", func(t *testing.T) {
		fn := Dispatcher(DispatcherConfig{
			CompressMinLength: 1,
		}, client)
		req := httptest.NewRequest(http.MethodPost, "/users/me", nil)
		req.Header.Set(pike.HeaderAcceptEncoding, "gzip")
		c := pike.NewContext(req)
		c.Identity = []byte("abc")
		c.Status = cache.Cacheable
		header := make(http.Header)
		header[pike.HeaderContentType] = []string{
			"application/json",
		}
		cr := &cache.Response{
			CreatedAt:  uint32(time.Now().Unix()),
			TTL:        300,
			StatusCode: 200,
			GzipBody:   []byte("ABCD"),
			Header:     header,
		}
		c.Resp = cr
		err := fn(c, func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("dispatch fail, %v", err)
		}
		if c.Response.Status() != 200 {
			t.Fatalf("the response code should be 200")
		}
		if string(c.Response.Bytes()) != "ABCD" {
			t.Fatalf("the response body should be ABCD")
		}
	})

	t.Run("dispatch gunzip response", func(t *testing.T) {
		fn := Dispatcher(DispatcherConfig{
			CompressMinLength: 1,
		}, client)
		req := httptest.NewRequest(http.MethodPost, "/users/me", nil)
		c := pike.NewContext(req)
		c.Identity = []byte("abc")
		c.Status = cache.Cacheable
		gzipBody, _ := util.Gzip([]byte("ABCD"), 0)
		cr := &cache.Response{
			CreatedAt:  uint32(time.Now().Unix()),
			TTL:        300,
			StatusCode: 200,
			GzipBody:   gzipBody,
		}
		c.Resp = cr
		err := fn(c, func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("dispatch fail, %v", err)
		}
		if c.Response.Status() != 200 {
			t.Fatalf("the response code should be 200")
		}
		if string(c.Response.Bytes()) != "ABCD" {
			t.Fatalf("the response body should be ABCD")
		}
	})
}
