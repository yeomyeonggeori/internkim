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
