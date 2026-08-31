import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { Link, useLocation } from 'react-router';
import { baseOptions } from '@/lib/layout.shared';
import { i18n, localeOfPath } from '@/lib/i18n';

const copy: Record<string, { title: string; body: string; back: string }> = {
  en: {
    title: 'Not Found',
    body: 'This page could not be found.',
    back: 'Back to the docs',
  },
  ko: {
    title: '페이지를 찾을 수 없습니다',
    body: '이 주소에는 아무것도 없습니다.',
    back: '문서로 돌아가기',
  },
};

export function meta() {
  return [{ title: copy[i18n.defaultLanguage].title }];
}

export default function NotFound() {
  const locale = localeOfPath(useLocation().pathname);
  const words = copy[locale] ?? copy[i18n.defaultLanguage];

  return (
    <HomeLayout {...baseOptions(locale)}>
      <div className="p-4 flex flex-col items-center justify-center text-center flex-1">
        <h1 className="text-xl font-bold mb-2">{words.title}</h1>
        <p className="text-fd-muted-foreground mb-4">{words.body}</p>
        <Link
          className="text-sm bg-fd-primary text-fd-primary-foreground rounded-full font-medium px-4 py-2.5"
          to={locale === i18n.defaultLanguage ? '/' : `/${locale}`}
        >
          {words.back}
        </Link>
      </div>
    </HomeLayout>
  );
}
