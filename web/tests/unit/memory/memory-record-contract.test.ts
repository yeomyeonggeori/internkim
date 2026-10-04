import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	factForgetRequest,
	memoryFactSchema,
	memoryFactsResponseSchema,
	memoryIndexSchema,
	memoryLayerSchema,
	memoryRecalledSchema,
	memoryRecallResponseSchema
} from '../../../src/routes/memory/memory-facts-api';

const blueclawSample: unknown = blueclawAnswer('memory_facts.json');
const blueclawRecallSample: unknown = blueclawAnswer('memory_recall.json');

function blueclawAnswer(name: string): unknown {
	return JSON.parse(readFileSync(new URL(`../../../../.dependency/blueclaw/internal/adminapi/testdata/${name}`, import.meta.url), 'utf8'));
}

function keysOf(document: unknown): string[] {
	return typeof document === 'object' && document !== null ? Object.keys(document) : [];
}

function arrayAt(document: unknown, key: string): unknown[] {
	if (typeof document !== 'object' || document === null || !(key in document)) return [];
	const value: unknown = Reflect.get(document, key);
	return Array.isArray(value) ? value : [];
}

function indexOf(document: unknown): unknown {
	return typeof document === 'object' && document !== null && 'index' in document ? document.index : undefined;
}

function fieldsAcross(documents: unknown[]): string[] {
	return [...new Set(documents.flatMap(keysOf))].sort();
}

describe('the memory blueclaw answers with', () => {
	test('is read whole by the schema the screen parses', () => {
		const sample = memoryFactsResponseSchema.parse(blueclawSample);
		expect(sample.facts.map((fact) => fact.scopeType).sort()).toEqual(['circle', 'person', 'workspace']);
		expect(sample.layers.map((layer) => layer.scopeType)).toEqual(['person', 'circle', 'workspace']);
	});

	test('names exactly the fields the screen knows, so a field added or renamed on either side fails here', () => {
		expect(keysOf(blueclawSample).sort()).toEqual(Object.keys(memoryFactsResponseSchema.shape).sort());
		expect(keysOf(indexOf(blueclawSample)).sort()).toEqual(Object.keys(memoryIndexSchema.shape).sort());
		expect(fieldsAcross(arrayAt(blueclawSample, 'facts'))).toEqual(Object.keys(memoryFactSchema.shape).sort());
		expect(fieldsAcross(arrayAt(blueclawSample, 'layers'))).toEqual(Object.keys(memoryLayerSchema.shape).sort());
	});

	test('answers a recall preview the screen reads whole, field for field', () => {
		expect(memoryRecallResponseSchema.parse(blueclawRecallSample).facts.length).toBeGreaterThan(0);
		expect(keysOf(blueclawRecallSample).sort()).toEqual(Object.keys(memoryRecallResponseSchema.shape).sort());
		expect(fieldsAcross(arrayAt(blueclawRecallSample, 'facts'))).toEqual(Object.keys(memoryRecalledSchema.shape).sort());
	});

	test('is forgotten by exact fact, with the reason only when given and never a reader identity', () => {
		expect(factForgetRequest(['fact:sample'], '  ')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample'] } });
		expect(factForgetRequest(['fact:sample', 'fact:other'], 'moved teams')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample', 'fact:other'], reason: 'moved teams' } });
	});
});
