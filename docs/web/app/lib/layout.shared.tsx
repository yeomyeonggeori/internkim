import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { defineI18nUI, type Translations } from 'fumadocs-ui/i18n';
import { i18n } from './i18n';
import { appName, gitConfig } from './shared';

const korean: Translations & { displayName: string } = {
  displayName: '한국어',
  'Ask AI(AI chat button)': 'AI에게 묻기',
  'Back to Home(404 not found page)': '처음으로',
  'Choose a language(language switcher)': '언어 선택',
  'Choose a language(language switcher)(aria-label)': '언어 선택',
  'Close Banner(banner)(aria-label)': '배너 닫기',
  'Close Search(search dialog)(aria-label)': '검색 닫기',
  'Close Sidebar(aria-label)': '사이드바 닫기',
  'Close Sidebar(sidebar)(aria-label)': '사이드바 닫기',
  'Collapse Sidebar(sidebar)(aria-label)': '사이드바 접기',
  'Copied Text(code block)(aria-label)': '복사했습니다',
  'Copy Anchor Link(heading anchor)(aria-label)': '이 항목 링크 복사',
  'Copy Link(accordion)(aria-label)': '링크 복사',
  'Copy Markdown(page actions)': 'Markdown 복사',
  'Copy Text(code block)(aria-label)': '복사',
  'Dark(theme switcher)(aria-label)': '어둡게',
  'Default(type table)': '기본값',
  'Edit on GitHub(edit page)': 'GitHub에서 편집',
  'Hide Sidebar(sidebar)': '사이드바 숨기기',
  'Last updated on(page footer)': '마지막 수정',
  'Layout Tab(layout tab trigger)': '레이아웃',
  'Light(theme switcher)(aria-label)': '밝게',
  'Next Page(pagination)': '다음',
  'No Headings(table of contents)': '제목 없음',
  'No results found(search dialog)': '결과가 없습니다',
  'On this page(table of contents)': '이 페이지에서',
  'Open Search(search trigger)(aria-label)': '검색 열기',
  'Open Sidebar(aria-label)': '사이드바 열기',
  'Open Sidebar(sidebar)(aria-label)': '사이드바 열기',
  'Open in ChatGPT(page actions)': 'ChatGPT에서 열기',
  'Open in Claude(page actions)': 'Claude에서 열기',
  'Open in Cursor(page actions)': 'Cursor에서 열기',
  'Open in GitHub(page actions)': 'GitHub에서 열기',
  'Open in Scira AI(page actions)': 'Scira AI에서 열기',
  'Open(page actions)': '열기',
  'Page Not Found(404 not found page)': '페이지를 찾을 수 없습니다',
  'Parameters(type table)': '매개변수',
  'Previous Page(pagination)': '이전',
  'Prop(type table)': '이름',
  'Read {url}, I want to ask questions about it.(page actions)':
    '{url}을 읽어 주세요. 그 내용을 두고 묻고 싶습니다.',
  'Returns(type table)': '반환값',
  'Search(search dialog)': '검색',
  'Search(search trigger)': '검색',
  'Show Sidebar(sidebar)': '사이드바 보이기',
  'System(theme switcher)(aria-label)': '시스템 설정을 따름',
  'Table of Contents(inline table of contents)': '목차',
  'The page you are looking for might have been removed, had its name changed, or is temporarily unavailable.(404 not found page)':
    '찾으시는 페이지가 사라졌거나, 이름이 바뀌었거나, 잠시 열 수 없는 상태입니다.',
  'Toggle Menu(home layout header)(aria-label)': '메뉴 열고 닫기',
  'Toggle Theme(theme switcher)(aria-label)': '테마 바꾸기',
  'Type(type table)': '타입',
  'View as Markdown(page actions)': 'Markdown으로 보기',
};

export const i18nUI = defineI18nUI(i18n, {
  en: { displayName: 'English' },
  ko: korean,
});

export function baseOptions(locale: string): BaseLayoutProps {
  return {
    i18n: true,
    nav: {
      title: (
        <span className="flex items-center gap-2">
          <img src="/logo.svg" alt="" className="size-6" />
          <span>{appName[locale] ?? appName[i18n.defaultLanguage]}</span>
        </span>
      ),
      url: locale === i18n.defaultLanguage ? '/' : `/${locale}`,
    },
    githubUrl: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
  };
}
