import catalog from '@/generated/tool-catalog.json';

type ReferenceLanguage = 'en' | 'ko';

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
    approval: 'Approval',
    presence: 'Presence',
    privacy: 'Privacy',
    version: 'Version',
    required: 'required',
    notRequired: 'not required',
    modelReads: 'What the model reads',
    endpoint: 'Call it over HTTP',
  },
  ko: {
    namespace: '네임스페이스',
    effect: '영향',
    approval: '승인',
    presence: '사용자 참석',
    privacy: '민감도',
    version: '버전',
    required: '필요',
    notRequired: '불필요',
    modelReads: '에이전트가 읽는 설명',
    endpoint: 'HTTP로 부르기',
  },
};

const koreanSideEffects: Record<string, string> = {
  read: '읽기',
  workspace_write: '워크스페이스 쓰기',
  external_write: '외부 수정',
  external_send: '외부 발송',
  site_publish: '사이트 게시',
  connect: '연결',
  destructive: '삭제',
};

function sideEffectLabel(sideEffectClass: string, language: ReferenceLanguage): string {
  if (language === 'ko') return koreanSideEffects[sideEffectClass] ?? sideEffectClass;
  return sideEffectClass;
}

function findTool(name: string): ReferenceTool | undefined {
  return (catalog.tools as ReferenceTool[]).find((tool) => tool.name === name);
}

export function ToolReference({ name, language }: { name: string; language: ReferenceLanguage }) {
  const tool = findTool(name);
  if (!tool) return null;

  const word = labels[language];
  const yesOrNo = (required: boolean) => (required ? word.required : word.notRequired);

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
            <th>{word.approval}</th>
            <td>{yesOrNo(tool.requiresApproval)}</td>
          </tr>
          <tr>
            <th>{word.presence}</th>
            <td>{yesOrNo(tool.requiresUserPresence)}</td>
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
        <a href={`${language === 'ko' ? '/ko' : ''}/docs/api/reference/tools/invoke_${tool.name}`}>
          <code>{`POST /tools/${tool.name}/invoke`}</code>
        </a>
      </p>
    </>
  );
}
