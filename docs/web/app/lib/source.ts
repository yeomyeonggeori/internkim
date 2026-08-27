import { loader } from 'fumadocs-core/source';
import { defineDocs } from 'fumadocs-mdx/macro';
import { i18n } from './i18n';
import { docsContentRoute, docsRoute } from './shared';

export const docs = defineDocs({
  dir: '..',
  docs: {
    files: ['*.{md,mdx}', 'tools/**/*.{md,mdx}', 'api/**/*.{md,mdx}'],
    async: true,
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
  meta: {
    files: ['*.json', 'tools/**/*.json', 'api/**/*.json'],
  },
});

export const source = loader({
  i18n,
  source: docs.toFumadocsSource(),
  baseUrl: docsRoute,
});

export function getPageMarkdownUrl(page: (typeof source)['$inferPage']) {
  const segments = [...page.slugs, 'content.md'];

  return {
    segments,
    url: '/' + [page.locale, ...docsContentRoute.split('/'), ...segments].filter(Boolean).join('/'),
  };
}

export async function getLLMText(page: (typeof source)['$inferPage']) {
  const processed = await page.data.getText('processed');

  return `# ${page.data.title} (${page.url})

${processed}`;
}
