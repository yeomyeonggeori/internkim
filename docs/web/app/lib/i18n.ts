import { defineI18n } from 'fumadocs-core/i18n';

export const i18n = defineI18n({
  defaultLanguage: 'en',
  languages: ['en', 'ko'],
  hideLocale: 'default-locale',
});

export function localeOfPath(pathname: string): string {
  const first = pathname.split('/').filter(Boolean)[0] ?? '';
  const languages: readonly string[] = i18n.languages;
  return languages.includes(first) ? first : i18n.defaultLanguage;
}

export function pathInLocale(pathname: string, locale: string): string {
  const segments = pathname.split('/').filter(Boolean);
  const languages: readonly string[] = i18n.languages;
  const withoutLocale = languages.includes(segments[0] ?? '') ? segments.slice(1) : segments;
  const prefixed = locale === i18n.defaultLanguage ? withoutLocale : [locale, ...withoutLocale];
  return `/${prefixed.join('/')}`;
}
