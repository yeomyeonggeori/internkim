import { mkdir, mkdtemp, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { protocolVersion } from '../src/lib/server/public-api/catalog/protocol';
import {
	buildProtocolManifest,
	buildSchemaArtifacts,
	calculateArtifactHash,
	serializeArtifact
} from '../src/lib/server/public-api/catalog/protocol-artifacts';

import { buildCapabilityToolCatalog } from '../src/lib/server/public-api/catalog/tools';

const generatedDirectory = fileURLToPath(new URL('../../pkg/capabilityprotocol/generated/', import.meta.url));
const catalogFileName = 'capability-tools.json';

export function buildGeneratedDocuments(): Map<string, string> {
	const schemas = buildSchemaArtifacts();
	const catalogDocument = serializeArtifact(buildCapabilityToolCatalog(protocolVersion));
	const manifest = buildProtocolManifest(schemas, {
		fileName: catalogFileName,
		hash: calculateArtifactHash(catalogDocument)
	});
	return new Map([
		[catalogFileName, catalogDocument],
		...schemas.map(({ fileName, schema }) => [join('json-schema', fileName), serializeArtifact(schema)] as const),
		['manifest.json', serializeArtifact(manifest)]
	]);
}

export async function writeGeneratedProtocol(targetDirectory = generatedDirectory): Promise<void> {
	const documents = buildGeneratedDocuments();
	const parentDirectory = dirname(targetDirectory.replace(/\/$/, ''));
	await mkdir(parentDirectory, { recursive: true });
	const stagingDirectory = await mkdtemp(join(parentDirectory, '.generated-staging-'));
	try {
		await writeDocuments(stagingDirectory, documents);
		await rm(targetDirectory, { force: true, recursive: true });
		await rename(stagingDirectory, targetDirectory.replace(/\/$/, ''));
	} finally {
		await rm(stagingDirectory, { force: true, recursive: true });
	}
}

export async function checkGeneratedProtocol(targetDirectory = generatedDirectory): Promise<void> {
	const documents = buildGeneratedDocuments();
	const expectedPaths = [...documents.keys()].sort();
	const actualPaths = await listRelativeFilePaths(targetDirectory);
	if (JSON.stringify(actualPaths) !== JSON.stringify(expectedPaths)) {
		throw new Error(`generated protocol paths differ: expected ${expectedPaths.join(', ')}, got ${actualPaths.join(', ')}`);
	}
	for (const [relativePath, expectedDocument] of documents) {
		const actualDocument = await readFile(join(targetDirectory, relativePath), 'utf8');
		if (actualDocument !== expectedDocument) {
			throw new Error(`generated protocol artifact is stale: ${relativePath}`);
		}
	}
}

async function writeDocuments(targetDirectory: string, documents: Map<string, string>): Promise<void> {
	await Promise.all(
		[...documents].map(async ([relativePath, document]) => {
			const artifactPath = join(targetDirectory, relativePath);
			await mkdir(dirname(artifactPath), { recursive: true });
			await writeFile(artifactPath, document);
		})
	);
}

async function listRelativeFilePaths(targetDirectory: string, relativeDirectory = ''): Promise<string[]> {
	const entries = await readdir(join(targetDirectory, relativeDirectory), { withFileTypes: true });
	const paths = await Promise.all(
		entries.map(async (entry) => {
			const relativePath = join(relativeDirectory, entry.name);
			if (!entry.isDirectory()) return [relativePath];
			return listRelativeFilePaths(targetDirectory, relativePath);
		})
	);
	return paths.flat().sort();
}

if (import.meta.main) {
	if (Bun.argv.includes('--check')) {
		await checkGeneratedProtocol();
	} else {
		await writeGeneratedProtocol();
	}
}
