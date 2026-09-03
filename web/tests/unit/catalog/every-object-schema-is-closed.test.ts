import { describe, expect, test } from 'bun:test';

import { buildCapabilityToolCatalog } from '../../../src/lib/server/public-api/catalog/tools';
import { protocolVersion } from '../../../src/lib/server/public-api/catalog/protocol';

// Mirrors validateExplicitlyClosedProviderSchemaObjects in
// bluecollar/toolcontract/provider.go, which quarantines a whole provider when
// one object schema at any depth leaves additionalProperties open.
function openObjectSchemasIn(node: unknown, trail: string): string[] {
	if (Array.isArray(node)) {
		return node.flatMap((item, index) => openObjectSchemasIn(item, `${trail}[${index}]`));
	}
	if (typeof node !== 'object' || node === null) return [];

	const schema = node as Record<string, unknown>;
	const open = describesAnObject(schema.type) && schema.additionalProperties !== false ? [trail] : [];
	const nested = Object.entries(schema).flatMap(([key, child]) =>
		openObjectSchemasIn(child, `${trail}.${key}`)
	);
	return [...open, ...nested];
}

function describesAnObject(declared: unknown): boolean {
	if (declared === 'object') return true;
	return Array.isArray(declared) && declared.includes('object');
}

describe('every schema the catalog offers is explicitly closed', () => {
	test('no object schema at any depth leaves additionalProperties open', () => {
		const catalog = buildCapabilityToolCatalog(protocolVersion);
		const open = catalog.tools.flatMap((tool) => [
			...openObjectSchemasIn(tool.inputSchema, `${tool.name}.inputSchema`),
			...openObjectSchemasIn(tool.inputIntentSchema, `${tool.name}.inputIntentSchema`),
			...openObjectSchemasIn(tool.outputSchema, `${tool.name}.outputSchema`),
			...openObjectSchemasIn(tool.resultContract?.schema, `${tool.name}.resultContract.schema`)
		]);

		expect(open).toEqual([]);
	});

	test('the walk finds an open object rather than passing everything', () => {
		const open = openObjectSchemasIn(
			{
				type: 'object',
				additionalProperties: false,
				properties: { labels: { type: 'object', additionalProperties: { type: 'string' } } }
			},
			'example'
		);

		expect(open).toEqual(['example.properties.labels']);
	});
});
