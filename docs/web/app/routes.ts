import { route, type RouteConfig } from '@react-router/dev/routes';

export default [
  route('api/search', 'routes/search.ts'),

  route('openapi/:language.json', 'routes/openapi.ts'),

  route('llms.txt', 'llms/index.ts'),
  route('llms-full.txt', 'llms/full.ts'),
  route('llms.mdx/docs/*', 'llms/mdx.ts'),

  route('*', 'routes/docs.tsx'),
] satisfies RouteConfig;
