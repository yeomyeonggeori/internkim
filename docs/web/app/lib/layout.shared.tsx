import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { defineI18nUI } from 'fumadocs-ui/i18n';
import { i18n } from './i18n';
import { appName, gitConfig } from './shared';

export const i18nUI = defineI18nUI(i18n, {
  en: { displayName: 'English' },
  ko: {
    displayName: '한국어',
    'Search(search dialog)': '검색',
    'Search(search trigger)': '검색',
    'On this page(table of contents)': '이 페이지에서',
    'Table of Contents(inline table of contents)': '목차',
    'No Headings(table of contents)': '제목 없음',
    'No results found(search dialog)': '결과가 없습니다',
    'Next Page(pagination)': '다음',
    'Previous Page(pagination)': '이전',
    'Last updated on(page footer)': '마지막 수정',
    'Choose a language(language switcher)': '언어 선택',
    'Copy Markdown(page actions)': 'Markdown 복사',
    'View as Markdown(page actions)': 'Markdown으로 보기',
    'Open(page actions)': '열기',
    'Edit on GitHub(edit page)': 'GitHub에서 편집',
    'Page Not Found(404 not found page)': '페이지를 찾을 수 없습니다',
    'Back to Home(404 not found page)': '처음으로',
    'Hide Sidebar(sidebar)': '사이드바 숨기기',
    'Show Sidebar(sidebar)': '사이드바 보이기',
  },
});

export function baseOptions(locale: string): BaseLayoutProps {
  return {
    i18n: true,
    nav: {
      title: appName,
      url: locale === i18n.defaultLanguage ? '/docs' : `/${locale}/docs`,
    },
    githubUrl: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
  };
}
