import { mkdir, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { generateFiles } from 'fumadocs-openapi';
import { createOpenAPI, type OpenAPIOptions } from 'fumadocs-openapi/server';
import { createOpenApiDocument, type ApiDocumentationLanguage } from '../../../web/src/lib/server/openapi';

type OperationItem = { path: string; method: string };
type PathsOfDocument = Record<string, Record<string, { operationId?: string } | undefined> | undefined>;

const languages: readonly ApiDocumentationLanguage[] = ['en', 'ko'];
const referenceDirectory = fileURLToPath(new URL('../../api/reference', import.meta.url));
const generatedDirectory = fileURLToPath(new URL('../app/generated', import.meta.url));

function sectionOf(item: OperationItem): string {
  const [, section] = item.path.split('/');
  return section ?? 'other';
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
    groupBy: (entry) => ('path' in entry.item ? sectionOf(entry.item as OperationItem) : 'other'),
    name: (output) => {
      const item = output.item as OperationItem;
      const identifier = identifiers.get(`${item.method} ${item.path}`) ?? `${sectionOf(item)}-${item.method}`;
      return language === 'ko' ? `${identifier}.ko` : identifier;
    },
  });
}

async function bundledDocument(language: ApiDocumentationLanguage, document: unknown): Promise<unknown> {
  const loaded = await serverFor(language, document).getSchema(language);
  return loaded.bundled;
}

await rm(referenceDirectory, { recursive: true, force: true });
await mkdir(generatedDirectory, { recursive: true });

const bundled: Record<string, unknown> = {};
for (const language of languages) {
  const document = createOpenApiDocument(language);
  await generatePages(language, document);
  bundled[language] = await bundledDocument(language, document);
}

await writeFile(`${generatedDirectory}/openapi.json`, JSON.stringify(bundled));

console.log(`wrote ${languages.length} OpenAPI documents and their operation pages`);
