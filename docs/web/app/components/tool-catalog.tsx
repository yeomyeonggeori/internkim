import catalog from '@/generated/tool-catalog.json';

type CatalogLanguage = 'en' | 'ko';

const koreanSideEffectLabels: Record<string, string> = {
  read: '읽기',
  workspace_write: '워크스페이스 쓰기',
  external_write: '외부 수정',
  external_send: '외부 발송',
  site_publish: '사이트 게시',
  connect: '연결',
  destructive: '삭제',
};

const columnHeadings: Record<CatalogLanguage, [string, string, string]> = {
  en: ['Tool', 'Effect', 'What the model reads'],
  ko: ['도구', '영향', '에이전트가 읽는 설명'],
};

function sideEffectLabel(sideEffectClass: string, language: CatalogLanguage): string {
  if (language === 'ko') {
    return koreanSideEffectLabels[sideEffectClass] ?? sideEffectClass;
  }
  return sideEffectClass;
}

export function ToolProtocolVersion() {
  return <>{catalog.protocolVersion}</>;
}

export function ToolCount() {
  return <>{catalog.tools.length}</>;
}

export function ToolCatalog({
  namespace,
  language = 'en',
}: {
  namespace?: string;
  language?: CatalogLanguage;
}) {
  const tools = namespace
    ? catalog.tools.filter((tool) => tool.namespace === namespace)
    : catalog.tools;
  const [toolHeading, effectHeading, descriptionHeading] = columnHeadings[language];

  return (
    <div className="overflow-x-auto">
      <table>
        <thead>
          <tr>
            <th>{toolHeading}</th>
            <th>{effectHeading}</th>
            <th>{descriptionHeading}</th>
          </tr>
        </thead>
        <tbody>
          {tools.map((tool) => (
            <tr key={tool.name}>
              <td>
                <code>{tool.name}</code>
              </td>
              <td>{language === 'en' ? <code>{tool.sideEffectClass}</code> : sideEffectLabel(tool.sideEffectClass, language)}</td>
              <td>{tool.summary}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
