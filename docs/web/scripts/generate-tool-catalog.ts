import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectDirectory = dirname(dirname(fileURLToPath(import.meta.url)));
const canonicalCatalogPath = join(
  projectDirectory,
  '../..',
  'pkg/capabilityprotocol/generated/capability-tools.json',
);
const generatedCatalogPath = join(projectDirectory, 'app/generated/tool-catalog.json');

type CanonicalTool = {
  name: string;
  namespace: string;
  description: string;
  sideEffectClass: string;
  requiresUserPresence: boolean;
  privacyClass: string;
};

type CanonicalCatalog = {
  protocolVersion: string;
  tools: CanonicalTool[];
};

function firstSentence(description: string): string {
  const [sentence] = description.replace(/\s+/g, ' ').split('. ');
  return sentence.endsWith('.') ? sentence : `${sentence}.`;
}

const canonicalCatalog: CanonicalCatalog = JSON.parse(await readFile(canonicalCatalogPath, 'utf8'));

const catalog = {
  protocolVersion: canonicalCatalog.protocolVersion,
  tools: canonicalCatalog.tools.map((tool) => ({
    name: tool.name,
    namespace: tool.namespace,
    summary: firstSentence(tool.description),
    sideEffectClass: tool.sideEffectClass,
    requiresUserPresence: tool.requiresUserPresence,
    privacyClass: tool.privacyClass,
  })),
};

await mkdir(dirname(generatedCatalogPath), { recursive: true });
await writeFile(generatedCatalogPath, `${JSON.stringify(catalog, null, 2)}\n`);

console.log(`wrote ${catalog.tools.length} tools from protocol ${catalog.protocolVersion}`);
