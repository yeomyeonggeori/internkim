import type { Route } from './+types/mdx';
import { getLLMText, source } from '@/lib/source';

function pageSlugsFromContentPath(contentPath: string): string[] {
  const segments = contentPath.split('/').filter((segment) => segment.length > 0);
  return segments.slice(0, -1);
}

export async function loader({ params }: Route.LoaderArgs) {
  const page = source.getPage(pageSlugsFromContentPath(params['*']));
  if (!page) {
    return new Response('not found', { status: 404 });
  }
  return new Response(await getLLMText(page), {
    headers: {
      'Content-Type': 'text/markdown',
    },
  });
}
