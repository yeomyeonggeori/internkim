import type { Route } from './+types/docs';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  ViewOptionsPopover,
} from 'fumadocs-ui/layouts/docs/page';
import { docs, getPageMarkdownUrl, source } from '@/lib/source';
import { baseOptions } from '@/lib/layout.shared';
import { i18n } from '@/lib/i18n';
import { gitConfig } from '@/lib/shared';
import { useFumadocsLoader } from 'fumadocs-core/source/client';
import { useMDXComponents } from '@/components/mdx';
import { use } from 'react';

export async function loader({ params }: Route.LoaderArgs) {
  const segments = params['*'].split('/').filter((v) => v.length > 0);
  const languages: readonly string[] = i18n.languages;
  const hasLanguagePrefix = languages.includes(segments[0] ?? '');
  const locale = hasLanguagePrefix ? segments[0] : i18n.defaultLanguage;
  const slugs = hasLanguagePrefix ? segments.slice(1) : segments;
  const page = source.getPage(slugs, locale);
  if (!page) throw new Response('Not found', { status: 404 });

  return {
    locale,
    path: page.path,
    markdownUrl: getPageMarkdownUrl(page).url,
    pageTree: await source.serializePageTree(source.getPageTree(locale)),
  };
}

function Content({ path, markdownUrl }: { path: string; markdownUrl: string }) {
  const page = docs.getPage(path);
  if (!page) throw new Error(`unknown page: ${path}`);

  const { toc } = use(page.load());
  const Mdx = page.body;

  return (
    <DocsPage toc={toc}>
      <title>{page.title}</title>
      <meta name="description" content={page.description} />
      <DocsTitle>{page.title}</DocsTitle>
      <DocsDescription>{page.description}</DocsDescription>
      <div className="flex flex-row gap-2 items-center border-b -mt-4 pb-6">
        <MarkdownCopyButton markdownUrl={markdownUrl} />
        <ViewOptionsPopover
          markdownUrl={markdownUrl}
          githubUrl={`https://github.com/${gitConfig.user}/${gitConfig.repo}/blob/${gitConfig.branch}/content/docs/${path}`}
        />
      </div>
      <DocsBody>
        <Mdx components={useMDXComponents()} />
      </DocsBody>
    </DocsPage>
  );
}

export default function Page({ loaderData }: Route.ComponentProps) {
  const { pageTree, path, markdownUrl, locale } = useFumadocsLoader(loaderData);

  return (
    <DocsLayout {...baseOptions(locale)} tree={pageTree}>
      <Content path={path} markdownUrl={markdownUrl} />
    </DocsLayout>
  );
}
