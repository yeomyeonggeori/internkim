import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	factForgetRequest,
	memoryFactSchema,
	memoryFactsResponseSchema,
	memoryIndexSchema
} from '../../../src/routes/memory/memory-facts-api';

const blueclawSample: unknown = JSON.parse(
	readFileSync(new URL('../../../../.dependency/blueclaw/internal/adminapi/testdata/memory_facts.json', import.meta.url), 'utf8')
);

function keysOf(document: unknown): string[] {
	return typeof document === 'object' && document !== null ? Object.keys(document) : [];
}

function factsOf(document: unknown): unknown[] {
	if (typeof document !== 'object' || document === null || !('facts' in document)) return [];
	return Array.isArray(document.facts) ? document.facts : [];
}

function indexOf(document: unknown): unknown {
	return typeof document === 'object' && document !== null && 'index' in document ? document.index : undefined;
}

describe('the memory blueclaw answers with', () => {
	test('is read whole by the schema the screen parses', () => {
		const sample = memoryFactsResponseSchema.parse(blueclawSample);
		expect(sample.facts.map((fact) => fact.scopeType).sort()).toEqual(['circle', 'person', 'workspace']);
	});

	test('names exactly the fields the screen knows, so a field added or renamed on either side fails here', () => {
		expect(keysOf(blueclawSample).sort()).toEqual(Object.keys(memoryFactsResponseSchema.shape).sort());
		expect(keysOf(indexOf(blueclawSample)).sort()).toEqual(Object.keys(memoryIndexSchema.shape).sort());
		expect([...new Set(factsOf(blueclawSample).flatMap(keysOf))].sort()).toEqual(Object.keys(memoryFactSchema.shape).sort());
	});

	test('is forgotten by exact fact, with the reason only when given and never a reader identity', () => {
		expect(factForgetRequest(['fact:sample'], '  ')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample'] } });
		expect(factForgetRequest(['fact:sample', 'fact:other'], 'moved teams')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample', 'fact:other'], reason: 'moved teams' } });
	});
});
