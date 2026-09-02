import catalog from '@/generated/tool-catalog.json';
import { approvalHeading, sideEffectLabel, yesOrNoLabel, type CatalogLanguage } from '@/lib/catalog-words';

type ReferenceLanguage = CatalogLanguage;

type ReferenceTool = {
  name: string;
  namespace: string;
  description: string;
  sideEffectClass: string;
  requiresApproval: boolean;
  requiresUserPresence: boolean;
  privacyClass: string;
  version: string;
};

const labels: Record<ReferenceLanguage, Record<string, string>> = {
  en: {
    namespace: 'Namespace',
    effect: 'Effect',
    presence: 'Presence',
    privacy: 'Privacy',
    version: 'Version',
    modelReads: 'What the model reads',
    endpoint: 'Call it over HTTP',
  },
  ko: {
    namespace: '네임스페이스',
    effect: '영향',
    presence: '사용자 참석',
    privacy: '민감도',
    version: '버전',
    modelReads: '에이전트가 읽는 설명',
    endpoint: 'HTTP로 부르기',
  },
};

function findTool(name: string): ReferenceTool | undefined {
  return (catalog.tools as ReferenceTool[]).find((tool) => tool.name === name);
}

export function ToolReference({ name, language }: { name: string; language: ReferenceLanguage }) {
  const tool = findTool(name);
  if (!tool) return null;

  const word = labels[language];

  return (
    <>
      <table>
        <tbody>
          <tr>
            <th>{word.namespace}</th>
            <td>
              <code>{tool.namespace}</code>
            </td>
          </tr>
          <tr>
            <th>{word.effect}</th>
            <td>{sideEffectLabel(tool.sideEffectClass, language)}</td>
          </tr>
          <tr>
            <th>{approvalHeading(language)}</th>
            <td>{yesOrNoLabel(tool.requiresApproval, language)}</td>
          </tr>
          <tr>
            <th>{word.presence}</th>
            <td>{yesOrNoLabel(tool.requiresUserPresence, language)}</td>
          </tr>
          <tr>
            <th>{word.privacy}</th>
            <td>
              <code>{tool.privacyClass}</code>
            </td>
          </tr>
          <tr>
            <th>{word.version}</th>
            <td>{`v${tool.version}`}</td>
          </tr>
        </tbody>
      </table>

      <h2 id="what-the-model-reads">{word.modelReads}</h2>
      <p>{tool.description}</p>

      <h2 id="over-http">{word.endpoint}</h2>
      <p>
        <a href={`${language === 'ko' ? '/ko' : ''}/docs/api/reference/${tool.namespace}/${tool.name}`}>
          <code>{`POST /tools/${tool.name}/invoke`}</code>
        </a>
      </p>
    </>
  );
}
