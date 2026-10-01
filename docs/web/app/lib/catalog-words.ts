export type CatalogLanguage = 'en' | 'ko';

const koreanSideEffectWords: Record<string, string> = {
  read: '읽기',
  workspace_write: '워크스페이스 쓰기',
  external_write: '외부 수정',
  external_send: '외부 발송',
  connect: '연결',
  destructive: '삭제',
};

const approvalHeadings: Record<CatalogLanguage, string> = { en: 'Approval', ko: '승인' };

const yesOrNoWords: Record<CatalogLanguage, [string, string]> = {
  en: ['required', 'not required'],
  ko: ['필요', '불필요'],
};

export function sideEffectLabel(sideEffectClass: string, language: CatalogLanguage): string {
  if (language === 'ko') return koreanSideEffectWords[sideEffectClass] ?? sideEffectClass;
  return sideEffectClass;
}

export function approvalHeading(language: CatalogLanguage): string {
  return approvalHeadings[language];
}

export function yesOrNoLabel(value: boolean, language: CatalogLanguage): string {
  const [yes, no] = yesOrNoWords[language];
  return value ? yes : no;
}
