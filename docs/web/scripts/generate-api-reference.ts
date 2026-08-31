import { mkdir, readdir, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { generateFiles } from 'fumadocs-openapi';
import { createOpenAPI, type OpenAPIOptions } from 'fumadocs-openapi/server';
import { baseTools, createOpenApiDocument, type ApiDocumentationLanguage } from '../app/lib/openapi';

type OperationItem = { path: string; method: string };
type PathsOfDocument = Record<string, Record<string, { operationId?: string } | undefined> | undefined>;

const languages: readonly ApiDocumentationLanguage[] = ['en', 'ko'];
const referenceDirectory = fileURLToPath(new URL('../../api/reference', import.meta.url));
const generatedDirectory = fileURLToPath(new URL('../app/generated', import.meta.url));

const namespaceByTool = new Map(baseTools().map((tool) => [tool.name, tool.namespace]));

// One token and the list of them are the same subject to a reader, so they read
// as one group however the paths divide them.
const groupByResource: Record<string, string> = { token: 'tokens' };

function groupOf(item: OperationItem): string {
  const [, resource, toolName] = item.path.split('/');
  if (resource !== 'tools') return groupByResource[resource ?? ''] ?? resource ?? 'other';
  return namespaceByTool.get(toolName ?? '') ?? 'discovery';
}

function operationIdsOf(document: unknown): Map<string, string> {
  const paths = ((document as { paths?: PathsOfDocument }).paths ?? {}) as PathsOfDocument;
  const identifiers = new Map<string, string>();
  for (const [path, methods] of Object.entries(paths)) {
    for (const [method, operation] of Object.entries(methods ?? {})) {
      if (operation?.operationId) identifiers.set(`${method} ${path}`, operation.operationId);
    }
  }
  return identifiers;
}

// fumadocs-openapi types a schema's `openapi` field as the literal '3.2.0';
// ours says 3.1.0 and its loader is what upgrades the version.
type LibrarySchema = Extract<NonNullable<OpenAPIOptions['input']>, Record<string, unknown>>[string];

function serverFor(language: ApiDocumentationLanguage, document: unknown) {
  return createOpenAPI({ input: { [language]: document as LibrarySchema } });
}

async function generatePages(language: ApiDocumentationLanguage, document: unknown): Promise<void> {
  const identifiers = operationIdsOf(document);
  await generateFiles({
    input: serverFor(language, document),
    output: referenceDirectory,
    per: 'operation',
    groupBy: (entry) => ('path' in entry.item ? groupOf(entry.item as OperationItem) : 'other'),
    name: (output) => {
      const item = output.item as OperationItem;
      const identifier = identifiers.get(`${item.method} ${item.path}`) ?? `${groupOf(item)}-${item.method}`;
      return language === 'ko' ? `${identifier}.ko` : identifier;
    },
  });
}

async function bundledDocument(language: ApiDocumentationLanguage, document: unknown): Promise<unknown> {
  const loaded = await serverFor(language, document).getSchema(language);
  return loaded.bundled;
}

// Every group under Reference is a section a reader scans, not a drawer they
// open one at a time. Reference itself still folds.
const koreanGroupTitles: Record<string, string> = {
  agent: '에이전트',
  artifact: '아티팩트',
  browser: '브라우저',
  calendar: '캘린더',
  channel: '채널',
  discovery: '도구 목록',
  document: '문서',
  files: '파일',
  image: '이미지',
  message: '메시지',
  person: '사람',
  site: '사이트',
  task: '업무',
  tokens: '토큰',
  web: '웹',
};

function englishGroupTitle(namespace: string): string {
  return namespace.replace(/(^|[-_])([a-z])/g, (_match, gap, letter) => (gap ? ' ' : '') + letter.toUpperCase());
}

function headingDocument(title: string): string {
  return `${JSON.stringify({ title, collapsible: false, defaultOpen: true }, null, 2)}\n`;
}

const sectionTitle = { en: 'Every endpoint', ko: '엔드포인트 하나하나' };

async function writeGroupHeadings(): Promise<void> {
  await writeFile(`${referenceDirectory}/meta.json`, `${JSON.stringify({ title: sectionTitle.en }, null, 2)}\n`);
  await writeFile(`${referenceDirectory}/meta.ko.json`, `${JSON.stringify({ title: sectionTitle.ko }, null, 2)}\n`);

  for (const entry of await readdir(referenceDirectory, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    await writeFile(`${referenceDirectory}/${entry.name}/meta.json`, headingDocument(englishGroupTitle(entry.name)));
    await writeFile(
      `${referenceDirectory}/${entry.name}/meta.ko.json`,
      headingDocument(koreanGroupTitles[entry.name] ?? englishGroupTitle(entry.name)),
    );
  }
}

await rm(referenceDirectory, { recursive: true, force: true });
await mkdir(generatedDirectory, { recursive: true });

const bundled: Record<string, unknown> = {};
for (const language of languages) {
  const document = createOpenApiDocument(language);
  await generatePages(language, document);
  bundled[language] = await bundledDocument(language, document);
}

await writeGroupHeadings();

await writeFile(`${generatedDirectory}/openapi.json`, JSON.stringify(bundled));

console.log(`wrote ${languages.length} OpenAPI documents and their operation pages`);
