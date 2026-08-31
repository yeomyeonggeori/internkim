import { index, route, type RouteConfig } from '@react-router/dev/routes';

export default [
  route('api/search', 'routes/search.ts'),

  route('openapi/:language.json', 'routes/openapi.ts'),

  route('llms.txt', 'llms/index.ts'),
  route('llms-full.txt', 'llms/full.ts'),
  route('llms.mdx/docs/*', 'llms/mdx.ts'),

  index('routes/docs.tsx', { id: 'docs-home' }),
  route('*', 'routes/docs.tsx'),
] satisfies RouteConfig;
