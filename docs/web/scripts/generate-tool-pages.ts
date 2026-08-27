import { mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

type CatalogTool = {
  name: string;
  namespace: string;
  summary: string;
};

type Catalog = { tools: CatalogTool[] };

const catalogPath = fileURLToPath(new URL('../app/generated/tool-catalog.json', import.meta.url));
const referenceDirectory = fileURLToPath(new URL('../../tools/reference', import.meta.url));

const sectionTitle = { en: 'Every tool', ko: '도구 하나하나' };

function pageSource(tool: CatalogTool, language: 'en' | 'ko'): string {
  return [
    '---',
    `title: ${tool.name}`,
    `description: ${JSON.stringify(tool.summary)}`,
    '---',
    '',
    `<ToolReference name="${tool.name}" language="${language}" />`,
    '',
  ].join('\n');
}

const catalog: Catalog = JSON.parse(await readFile(catalogPath, 'utf8'));

await rm(referenceDirectory, { recursive: true, force: true });
await mkdir(referenceDirectory, { recursive: true });

for (const tool of catalog.tools) {
  await writeFile(`${referenceDirectory}/${tool.name}.mdx`, pageSource(tool, 'en'));
  await writeFile(`${referenceDirectory}/${tool.name}.ko.mdx`, pageSource(tool, 'ko'));
}

await writeFile(`${referenceDirectory}/meta.json`, `${JSON.stringify({ title: sectionTitle.en }, null, 2)}\n`);
await writeFile(`${referenceDirectory}/meta.ko.json`, `${JSON.stringify({ title: sectionTitle.ko }, null, 2)}\n`);

console.log(`wrote ${catalog.tools.length} tool pages in two languages`);
