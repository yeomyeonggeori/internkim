import type { Config } from '@react-router/dev/config';
import { glob } from 'node:fs/promises';
import { createGetUrl, getSlugs } from 'fumadocs-core/source';
import { i18n } from './app/lib/i18n';

const publishedContentDirectory = '..';
// kept in step with `docs.files` in app/lib/source.ts by hand: fumadocs-mdx rejects a
// non-literal `files` in a macro, and this config cannot import a macro module.
const publishedSections = ['*.{md,mdx}', 'tools/**/*.{md,mdx}', 'api/**/*.{md,mdx}'];

function localePrefix(language: string): string {
  return language === i18n.defaultLanguage ? '' : `/${language}`;
}

function entryLanguage(entry: string): string {
  const [, language] = entry.replace(/\.mdx?$/, '').match(/\.([a-z]{2})$/) ?? [];
  const languages: readonly string[] = i18n.languages;
  return language && languages.includes(language) ? language : i18n.defaultLanguage;
}

function entrySlugs(entry: string): string[] {
  return getSlugs(entry.replace(/\.[a-z]{2}(\.mdx?)$/, '$1'));
}

function pageURL(language: string, slugs: string[]): string {
  const url = `${localePrefix(language)}${createGetUrl('/')(slugs)}`;
  return url.length > 1 && url.endsWith('/') ? url.slice(0, -1) : url;
}

export default {
  ssr: false,
  async prerender({ getStaticPaths }) {
    const paths: string[] = [...getStaticPaths()];

    for await (const entry of glob(publishedSections, { cwd: publishedContentDirectory })) {
      const language = entryLanguage(entry);
      const slugs = entrySlugs(entry);
      paths.push(pageURL(language, slugs));
      if (language === i18n.defaultLanguage) {
        paths.push(`/llms.mdx/docs/${[...slugs, 'content.md'].join('/')}`);
      }
    }

    paths.push('/openapi/en.json', '/openapi/ko.json');

    return paths;
  },
} satisfies Config;
