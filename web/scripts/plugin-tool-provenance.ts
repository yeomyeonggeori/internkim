import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';

import { fullPublicAPIPermission } from '../src/lib/public-api-permission';
import {
	toolsAModelReachesWith,
	type ToolDescriptor
} from '../src/lib/server/public-api/catalog';
import { buildCapabilityToolCatalog } from '../src/lib/server/public-api/catalog/tools';
import { protocolVersion } from '../src/lib/server/public-api/catalog/protocol';

export type PluginSkillSource = { name: string; document: string };

export type PluginToolProvenanceReport = {
	plugin: { name: string; version: string; skillCount: number };
	catalog: {
		modelVisibleFullPermission: number;
		answeredBy: Record<'record' | 'company' | 'local', number>;
		modelSchemaBytes: number;
	};
	skills: Array<{
		name: string;
		references: string[];
		serviceReferences: string[];
		localReferences: string[];
		unresolvedReferences: string[];
	}>;
	references: { service: string[]; local: string[]; unresolved: string[] };
	summary: {
		skillCount: number;
		referenceCount: number;
		serviceReferenceCount: number;
		localReferenceCount: number;
		unresolvedReferenceCount: number;
	};
};

type PluginSource = {
	name: string;
	version: string;
	skills: PluginSkillSource[];
};

export type OwnershipDescriptor = Pick<ToolDescriptor, 'name' | 'answeredBy'>;
type SchemaDescriptor = Pick<ToolDescriptor, 'name' | 'inputSchema' | 'outputSchema'>;
type ReferenceBuckets = Record<'service' | 'local' | 'unresolved', string[]>;

function unique(values: string[]): string[] {
	return [...new Set(values)].sort();
}

export function parseToolReferences(document: string): string[] {
	const lines = document.split(/\r?\n/);
	if (lines[0] !== '---') return [];
	const closingMarker = lines.findIndex((line, index) => index > 0 && line === '---');
	if (closingMarker < 0) throw new Error('skill frontmatter is not closed');
	const frontmatter: unknown = Bun.YAML.parse(lines.slice(1, closingMarker).join('\n'));
	if (!isRecord(frontmatter)) throw new Error('skill frontmatter must be a mapping');
	const metadata = frontmatter.metadata;
	if (metadata === undefined) return [];
	if (!isRecord(metadata)) throw new Error('skill metadata must be a mapping');
	const references = metadata['kim.intern.tool-references'];
	if (references === undefined) return [];
	if (typeof references !== 'string') throw new Error('skill tool-references must be a string');
	return unique(references.trim().split(/\s+/).filter(Boolean));
}

function classifyReference(
	reference: string,
	serviceNames: ReadonlySet<string>,
	localNames: ReadonlySet<string>
): 'service' | 'local' | 'unresolved' {
	if (serviceNames.has(reference)) return 'service';
	if (localNames.has(reference)) return 'local';
	return 'unresolved';
}

function descriptorCounts(descriptors: OwnershipDescriptor[]): Record<'record' | 'company' | 'local', number> {
	return {
		record: descriptors.filter((descriptor) => descriptor.answeredBy === 'record').length,
		company: descriptors.filter((descriptor) => descriptor.answeredBy === 'company').length,
		local: descriptors.filter((descriptor) => descriptor.answeredBy === 'local').length
	};
}

export function buildPluginToolProvenanceReport(
	source: PluginSource,
	serviceDescriptors: OwnershipDescriptor[],
	allDescriptors: OwnershipDescriptor[],
	modelSchemaBytes: number
): PluginToolProvenanceReport {
	const serviceNames = new Set(serviceDescriptors.map((descriptor) => descriptor.name));
	const localNames = new Set(
		allDescriptors.filter((descriptor) => descriptor.answeredBy === 'local').map((descriptor) => descriptor.name)
	);
	const skills = source.skills.map(({ name, document }) => {
		const references = parseToolReferences(document);
		const classified: ReferenceBuckets = { service: [], local: [], unresolved: [] };
		for (const reference of references) {
			const ownership = classifyReference(reference, serviceNames, localNames);
			classified[ownership].push(reference);
		}
		return {
			name,
			references,
			serviceReferences: classified.service,
			localReferences: classified.local,
			unresolvedReferences: classified.unresolved
		};
	});
	const references = {
		service: unique(skills.flatMap((skill) => skill.serviceReferences)),
		local: unique(skills.flatMap((skill) => skill.localReferences)),
		unresolved: unique(skills.flatMap((skill) => skill.unresolvedReferences))
	};

	return {
		plugin: { name: source.name, version: source.version, skillCount: skills.length },
		catalog: {
			modelVisibleFullPermission: serviceDescriptors.length,
			answeredBy: descriptorCounts(allDescriptors),
			modelSchemaBytes
		},
		skills,
		references,
		summary: {
			skillCount: skills.length,
			referenceCount: new Set(skills.flatMap((skill) => skill.references)).size,
			serviceReferenceCount: references.service.length,
			localReferenceCount: references.local.length,
			unresolvedReferenceCount: references.unresolved.length
		}
	};
}

async function loadPluginSource(repositoryRoot: string): Promise<PluginSource> {
	const dependencyRoot = join(repositoryRoot, '.dependency');
	const dependencyNames = await readdir(dependencyRoot);
	for (const dependencyName of dependencyNames) {
		const dependencyPath = join(dependencyRoot, dependencyName);
		const manifestPath = join(dependencyPath, 'plugin.json');
		let manifestDocument: string;
		try {
			manifestDocument = await readFile(manifestPath, 'utf8');
		} catch (errorValue) {
			if (isNodeError(errorValue) && errorValue.code === 'ENOENT') continue;
			throw errorValue;
		}
		const manifest = parsePluginManifest(manifestDocument);
		if (manifest === undefined || manifest.name !== 'internkim') continue;
		if (manifest.version === undefined) throw new Error('internkim plugin manifest must declare a string version');
		const skillsRoot = join(dependencyPath, 'skills');
		const skillNames = await readdir(skillsRoot);
		const skills = await Promise.all(
			skillNames.sort().map(async (skillName) => ({
				name: skillName,
				document: await readFile(join(skillsRoot, skillName, 'SKILL.md'), 'utf8')
			}))
		);
		return { name: manifest.name, version: manifest.version, skills };
	}
	throw new Error('internkim plugin manifest was not found');
}

function isNodeError(errorValue: unknown): errorValue is NodeJS.ErrnoException {
	return errorValue instanceof Error && 'code' in errorValue;
}

function parsePluginManifest(document: string): { name: string; version?: string } | undefined {
	let parsedDocument: unknown;
	try {
		parsedDocument = JSON.parse(document);
	} catch {
		return undefined;
	}
	if (!isRecord(parsedDocument) || typeof parsedDocument.name !== 'string') return undefined;
	if (parsedDocument.version === undefined) return { name: parsedDocument.name };
	if (typeof parsedDocument.version !== 'string') {
		throw new Error(`plugin ${parsedDocument.name} manifest must declare a string version`);
	}
	return { name: parsedDocument.name, version: parsedDocument.version };
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return value !== null && typeof value === 'object' && !Array.isArray(value);
}

export async function readPluginToolProvenanceReport(repositoryRoot: string): Promise<PluginToolProvenanceReport> {
	const source = await loadPluginSource(repositoryRoot);
	const allDescriptors = buildCapabilityToolCatalog(protocolVersion).tools;
	const serviceDescriptors = toolsAModelReachesWith(fullPublicAPIPermission);
	const ownershipDescriptors = allDescriptors.map(({ name, answeredBy }) => ({ name, answeredBy }));
	const modelSchemaBytes = Buffer.byteLength(
		JSON.stringify(
			serviceDescriptors.map(({ name, inputSchema, outputSchema }: SchemaDescriptor) => ({
				name,
				inputSchema,
				outputSchema
			}))
		)
	);
	return buildPluginToolProvenanceReport(source, serviceDescriptors, ownershipDescriptors, modelSchemaBytes);
}

if (import.meta.main) {
	const repositoryRoot = join(import.meta.dir, '../..');
	const report = await readPluginToolProvenanceReport(repositoryRoot);
	if (process.argv.includes('--json')) {
		console.log(JSON.stringify(report, null, 2));
	} else {
		console.log(`plugin ${report.plugin.name}@${report.plugin.version}`);
		console.log(`skills ${report.summary.skillCount}; references ${report.summary.referenceCount}`);
		console.log(
			`service ${report.summary.serviceReferenceCount}; local ${report.summary.localReferenceCount}; ` +
			`unresolved ${report.summary.unresolvedReferenceCount}`
		);
		if (report.references.unresolved.length > 0) console.log(`unresolved: ${report.references.unresolved.join(', ')}`);
	}
}
