import { describe, expect, test } from 'bun:test';
import { filterMemoryFacts, isLiveMemoryFact, memoryAudience, memoryKindLabel } from '../../../src/routes/memory/memory-workbench-model';
import type { MemoryFact } from '../../../src/routes/memory/memory-facts-api';
import { memoryText } from '../../../src/routes/memory/text';

const fact: MemoryFact = {
	factID: 'fact:sample',
	episodeID: 'episode:sample',
	ownerPersonID: 'person:sample',
	circleIDs: [],
	kind: 'fact',
	content: 'A useful decision',
	validFrom: '2026-08-01T00:00:00Z',
	reinforcementCount: 1,
	triggerPhrases: []
};
const now = Date.parse('2026-09-01T00:00:00Z');

describe('the remembered facts people inspect', () => {
	test('live means valid now: not before validFrom, not at or after validUntil', () => {
		expect(isLiveMemoryFact(fact, now)).toBe(true);
		expect(isLiveMemoryFact({ ...fact, validFrom: '2026-09-02T00:00:00Z' }, now)).toBe(false);
		expect(isLiveMemoryFact({ ...fact, validUntil: '2026-09-02T00:00:00Z' }, now)).toBe(true);
		expect(isLiveMemoryFact({ ...fact, validUntil: '2026-09-01T00:00:00Z' }, now)).toBe(false);
		expect(isLiveMemoryFact({ ...fact, validUntil: '2026-08-31T00:00:00Z' }, now)).toBe(false);
	});

	test('search filters by content, kind and circle without asking the plane', () => {
		const facts = [fact, { ...fact, factID: 'fact:shared', circleIDs: ['member'], kind: 'preference' as const, content: 'Short release notes' }];
		expect(filterMemoryFacts(facts, '')).toEqual(facts);
		expect(filterMemoryFacts(facts, 'DECISION').map((item) => item.factID)).toEqual(['fact:sample']);
		expect(filterMemoryFacts(facts, 'preference').map((item) => item.factID)).toEqual(['fact:shared']);
		expect(filterMemoryFacts(facts, 'member').map((item) => item.factID)).toEqual(['fact:shared']);
		expect(filterMemoryFacts(facts, 'nothing here')).toEqual([]);
	});

	test('audience names the circles a fact is shared with, or calls it mine', () => {
		expect(memoryAudience(fact, memoryText.ko)).toBe('내 기억');
		expect(memoryAudience({ ...fact, circleIDs: ['member', 'hr'] }, memoryText.en)).toBe('shared with circle · member, hr');
	});

	test('every fact kind has a label in both languages', () => {
		for (const kind of ['identity', 'preference', 'fact', 'episode', 'temporary'] as const) {
			expect(memoryKindLabel(kind, memoryText.ko)).not.toBe('');
			expect(memoryKindLabel(kind, memoryText.en)).not.toBe('');
		}
	});
});
