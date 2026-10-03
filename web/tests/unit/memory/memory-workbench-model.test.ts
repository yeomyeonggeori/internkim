import { describe, expect, test } from 'bun:test';
import { filterMemoryFacts, isCurrentMemory, memoryScopeLabel, memoryWhen } from '../../../src/routes/memory/memory-workbench-model';
import type { MemoryFact } from '../../../src/routes/memory/memory-facts-api';
import { memoryText } from '../../../src/routes/memory/text';

const fact: MemoryFact = {
	factID: 'fact:sample',
	originID: 'origin:sample',
	scopeType: 'person',
	isStatic: true,
	content: 'A useful decision',
	importance: 3,
	storageStrength: 1,
	createdAt: '2026-08-01T00:00:00Z',
	triggerPhrases: []
};
const leadership = { id: 'leadership', name: 'Leadership', nameKO: '경영진', readableCategories: [] };
const now = Date.parse('2026-09-01T00:00:00Z');

describe('the remembered facts people inspect', () => {
	test('current means warm and not past its validity', () => {
		expect(isCurrentMemory(fact, now)).toBe(true);
		expect(isCurrentMemory({ ...fact, validUntil: '2026-09-02T00:00:00Z' }, now)).toBe(true);
		expect(isCurrentMemory({ ...fact, validUntil: '2026-09-01T00:00:00Z' }, now)).toBe(false);
		expect(isCurrentMemory({ ...fact, coldSince: '2026-08-15T00:00:00Z' }, now)).toBe(false);
	});

	test('a scope reads as mine, the circle by its name, or the whole company', () => {
		expect(memoryScopeLabel(fact, [leadership], memoryText.ko, 'ko')).toBe('내 기억');
		expect(memoryScopeLabel({ ...fact, scopeType: 'circle', scopeID: 'leadership' }, [leadership], memoryText.ko, 'ko')).toBe('서클 · 경영진');
		expect(memoryScopeLabel({ ...fact, scopeType: 'circle', scopeID: 'leadership' }, [leadership], memoryText.en, 'en')).toBe('Circle · Leadership');
		expect(memoryScopeLabel({ ...fact, scopeType: 'circle', scopeID: 'finance' }, [], memoryText.en, 'en')).toBe('Circle · finance');
		expect(memoryScopeLabel({ ...fact, scopeType: 'workspace' }, [], memoryText.ko, 'ko')).toBe('회사 공용');
	});

	test('a fact that always holds has no date; something that happened has its own', () => {
		expect(memoryWhen(fact, memoryText.ko, 'ko')).toBe('늘 기억함');
		expect(memoryWhen({ ...fact, isStatic: false }, memoryText.ko, 'ko')).toBe('있었던 일');
		const happened = memoryWhen({ ...fact, isStatic: false, occurredAt: '2026-10-05T00:00:00Z' }, memoryText.en, 'en');
		expect(happened).toContain('2026');
		expect(happened).not.toContain('→');
	});

	test('search filters by content, scope and the phrases that recall it', () => {
		const shared: MemoryFact = { ...fact, factID: 'fact:circle', scopeType: 'circle', scopeID: 'leadership', content: 'Board meets Monday', triggerPhrases: ['이사회 일정'] };
		const label = (item: MemoryFact) => memoryScopeLabel(item, [leadership], memoryText.ko, 'ko');
		expect(filterMemoryFacts([fact, shared], '', label)).toEqual([fact, shared]);
		expect(filterMemoryFacts([fact, shared], 'DECISION', label).map((item) => item.factID)).toEqual(['fact:sample']);
		expect(filterMemoryFacts([fact, shared], '경영진', label).map((item) => item.factID)).toEqual(['fact:circle']);
		expect(filterMemoryFacts([fact, shared], '이사회', label).map((item) => item.factID)).toEqual(['fact:circle']);
	});
});
