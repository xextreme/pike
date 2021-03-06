import Token from '../views/token';
import Director from '../views/director';
import Cached from '../views/cached';
import Performance from '../views/performance';
import Fetching from '../views/fetching';

export default [
  {
    name: 'token',
    path: '/token',
    component: Token,
  },
  {
    name: 'director',
    path: '/',
    component: Director,
  },
  {
    name: 'cached',
    path: '/cached',
    component: Cached,
  },
  {
    name: 'performance',
    path: '/performance',
    component: Performance,
  },
  {
    name: 'fetching',
    path: '/fetching',
    component: Fetching,
  },
];
