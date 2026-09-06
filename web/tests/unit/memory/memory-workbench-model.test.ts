import { describe, expect, test } from 'bun:test';
import { isCurrentMemoryFact, memoryFactKey, memorySources } from '../../../src/routes/memory/memory-workbench-model';
import type { MemoryGraphFact } from '../../../src/routes/memory/memory-graph-api';

const fact: MemoryGraphFact = { factID: 'fact:sample', namespaceID: 'user:sample', scopeType: 'user', content: 'A useful decision' };
const now = Date.parse('2026-09-01T00:00:00Z');

describe('the remembered facts people inspect', () => {
	test('current means valid now, including future invalidation and past expiry', () => {
		expect(isCurrentMemoryFact(fact, now)).toBe(true);
		expect(isCurrentMemoryFact({ ...fact, validAt: '2026-09-02T00:00:00Z' }, now)).toBe(false);
		expect(isCurrentMemoryFact({ ...fact, invalidAt: '2026-09-02T00:00:00Z' }, now)).toBe(true);
		expect(isCurrentMemoryFact({ ...fact, invalidAt: '2026-08-31T00:00:00Z' }, now)).toBe(false);
		expect(isCurrentMemoryFact({ ...fact, expiredAt: '2026-09-01T00:00:00Z' }, now)).toBe(false);
	});

	test('selection stays distinct across namespace boundaries', () => {
		expect(memoryFactKey(fact)).not.toBe(memoryFactKey({ ...fact, namespaceID: 'user:other' }));
	});

	test('only recorded source IDs resolve, even when a fact ID matches an episode', () => {
		const episodes = [{ episodeID: fact.factID }, { episodeID: 'source:1' }, { episodeID: 'source:2' }];
		expect(memorySources(fact, episodes)).toEqual([]);
		expect(memorySources({ ...fact, sourceEpisodeIDs: ['source:1', 'source:2'] }, episodes)).toEqual(episodes.slice(1));
	});
});
