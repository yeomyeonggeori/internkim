import catalog from '@/generated/tool-catalog.json';
import { approvalHeading, sideEffectLabel, yesOrNoLabel, type CatalogLanguage } from '@/lib/catalog-words';

const columnHeadings: Record<CatalogLanguage, [string, string, string]> = {
  en: ['Tool', 'Effect', 'What the model reads'],
  ko: ['도구', '영향', '에이전트가 읽는 설명'],
};

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
            <th>{approvalHeading(language)}</th>
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
              <td>{yesOrNoLabel(tool.requiresApproval, language)}</td>
              <td>{tool.summary}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
