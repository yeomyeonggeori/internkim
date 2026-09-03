import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

type RecordFunction = {
  name: string;
  arguments: string;
  returns: string;
  security: 'definer' | 'invoker';
  callableBy: string[];
};

type EdgeFunction = { name: string; verifyJWT: boolean | null };
type ScheduledJob = { schedule: string; command: string };

type RecordSurface = {
  rpc: RecordFunction[];
  triggerFunctions: string[];
  edgeFunctions: EdgeFunction[];
  scheduled: ScheduledJob[];
};

type Language = 'en' | 'ko';

const surfacePath = fileURLToPath(new URL('../../../supabase/generated/record-surface.json', import.meta.url));
const recordDirectory = fileURLToPath(new URL('../../record', import.meta.url));

const words = {
  en: {
    rpcTitle: 'Every RPC',
    rpcDescription: 'Every function PostgREST exposes at /rest/v1/rpc, and who may call it.',
    rpcLead:
      'The record answers these directly. A caller reaches one at `POST /rest/v1/rpc/<name>` on the company\'s Supabase, running as whichever role its key carries. Row level security still applies to everything a function touches, and a `definer` function additionally runs with its owner\'s privileges, so its own body is the only thing deciding what the caller may see.',
    triggerLead: 'These run when a row changes rather than when somebody calls them.',
    edgeTitle: 'Edge functions',
    edgeDescription: 'The Deno functions deployed beside the record, and whether each verifies a JWT.',
    edgeLead:
      'These run on Supabase rather than in the web app. A function whose `verify_jwt` is false answers an unauthenticated request, so its own body is what decides who is allowed.',
    scheduledTitle: 'Scheduled work',
    scheduledDescription: 'What pg_cron runs, and how often.',
    scheduledLead: 'The record does this on its own clock, with no caller.',
    columns: { name: 'Name', arguments: 'Arguments', returns: 'Returns', security: 'Security', callableBy: 'Callable by' },
    edgeColumns: { name: 'Name', verifyJWT: 'Verifies JWT' },
    scheduledColumns: { schedule: 'Schedule', command: 'Runs' },
    triggerHeading: 'Trigger functions',
    nobody: 'nobody',
    defaults: 'CLI default',
  },
  ko: {
    rpcTitle: 'RPC 전체',
    rpcDescription: 'PostgREST가 /rest/v1/rpc로 노출하는 함수 전부와, 누가 부를 수 있는지.',
    rpcLead:
      '레코드가 직접 답하는 것들입니다. 회사의 Supabase에서 `POST /rest/v1/rpc/<name>`으로 도달하며, 호출자의 키가 지닌 역할로 실행됩니다. 함수가 건드리는 모든 것에 행 수준 보안이 그대로 적용되고, `definer` 함수는 여기에 더해 소유자의 권한으로 실행되므로 호출자가 무엇을 볼 수 있는지는 그 함수의 본문만이 정합니다.',
    triggerLead: '누군가 부를 때가 아니라 행이 바뀔 때 실행됩니다.',
    edgeTitle: '엣지 함수',
    edgeDescription: '레코드 옆에 배포된 Deno 함수와, 각각이 JWT를 검증하는지.',
    edgeLead:
      '웹 앱이 아니라 Supabase에서 실행됩니다. `verify_jwt`가 false인 함수는 인증 없는 요청에 답하므로, 누구를 허용할지는 그 함수의 본문이 정합니다.',
    scheduledTitle: '예약 실행',
    scheduledDescription: 'pg_cron이 무엇을 얼마나 자주 실행하는지.',
    scheduledLead: '호출자 없이 레코드가 자기 시계로 실행합니다.',
    columns: { name: '이름', arguments: '인자', returns: '반환', security: '보안', callableBy: '호출 가능' },
    edgeColumns: { name: '이름', verifyJWT: 'JWT 검증' },
    scheduledColumns: { schedule: '주기', command: '실행' },
    triggerHeading: '트리거 함수',
    nobody: '없음',
    defaults: 'CLI 기본값',
  },
} as const;

function frontMatter(title: string, description: string): string[] {
  return ['---', `title: ${title}`, `description: ${JSON.stringify(description)}`, '---', ''];
}

function code(value: string): string {
  return value.trim() === '' ? '—' : `\`${value.replace(/\|/g, '\\|')}\``;
}

function table(headers: string[], rows: string[][]): string[] {
  return [
    `| ${headers.join(' | ')} |`,
    `| ${headers.map(() => '---').join(' | ')} |`,
    ...rows.map((row) => `| ${row.join(' | ')} |`),
    '',
  ];
}

function rpcPage(surface: RecordSurface, language: Language): string {
  const said = words[language];
  return [
    ...frontMatter(said.rpcTitle, said.rpcDescription),
    said.rpcLead,
    '',
    ...table(
      [said.columns.name, said.columns.arguments, said.columns.returns, said.columns.security, said.columns.callableBy],
      surface.rpc.map((entry) => [
        code(entry.name),
        code(entry.arguments),
        code(entry.returns),
        entry.security,
        entry.callableBy.length === 0 ? said.nobody : entry.callableBy.join(', '),
      ]),
    ),
    `## ${said.triggerHeading}`,
    '',
    said.triggerLead,
    '',
    ...table([said.columns.name], surface.triggerFunctions.map((name) => [code(name)])),
  ].join('\n');
}

function edgePage(surface: RecordSurface, language: Language): string {
  const said = words[language];
  return [
    ...frontMatter(said.edgeTitle, said.edgeDescription),
    said.edgeLead,
    '',
    ...table(
      [said.edgeColumns.name, said.edgeColumns.verifyJWT],
      surface.edgeFunctions.map((entry) => [
        code(entry.name),
        entry.verifyJWT === null ? said.defaults : String(entry.verifyJWT),
      ]),
    ),
  ].join('\n');
}

function scheduledPage(surface: RecordSurface, language: Language): string {
  const said = words[language];
  return [
    ...frontMatter(said.scheduledTitle, said.scheduledDescription),
    said.scheduledLead,
    '',
    ...table(
      [said.scheduledColumns.schedule, said.scheduledColumns.command],
      surface.scheduled.map((job) => [code(job.schedule), code(job.command)]),
    ),
  ].join('\n');
}

const surface: RecordSurface = JSON.parse(await readFile(surfacePath, 'utf8'));

await mkdir(recordDirectory, { recursive: true });
for (const language of ['en', 'ko'] as Language[]) {
  const suffix = language === 'en' ? '' : '.ko';
  await writeFile(`${recordDirectory}/rpc${suffix}.mdx`, rpcPage(surface, language));
  await writeFile(`${recordDirectory}/edge-functions${suffix}.mdx`, edgePage(surface, language));
  await writeFile(`${recordDirectory}/scheduled${suffix}.mdx`, scheduledPage(surface, language));
}

console.log(
  `wrote ${surface.rpc.length} rpc, ${surface.triggerFunctions.length} trigger functions, ${surface.edgeFunctions.length} edge functions, ${surface.scheduled.length} scheduled jobs in two languages`,
);
