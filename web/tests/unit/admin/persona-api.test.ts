import { describe, expect, test } from 'bun:test';
import { readAgentIdentity } from '../../../src/routes/settings/persona-api';

describe('agent identity schema', () => {
	test('preserves canonical names and editable fields', () => {
		expect(readAgentIdentity({ schemaVersion: 1, names: ['샘플'], handle: 'sample', creature: 'helper', role: '업무 지원', emoji: '🌱', introduction: '안녕하세요.' })).toEqual({ schemaVersion: 1, names: ['샘플'], handle: 'sample', creature: 'helper', role: '업무 지원', emoji: '🌱', introduction: '안녕하세요.' });
	});

	test('rejects missing primary name', () => {
		expect(() => readAgentIdentity({ schemaVersion: 1, names: [] })).toThrow();
	});
});
