import { describe, expect, test } from 'bun:test';
import { factForgetRequest, normalizeMemoryFactsResponse } from '../../../src/routes/memory/memory-facts-api';

describe('memory record evidence', () => {
	test('preserves validity, reinforcement and recall independently of one another', () => {
		const response = normalizeMemoryFactsResponse({
			personID: 'person:sample',
			profile: { personID: 'person:sample', identityLines: ['Works on the platform team'], currentLines: [], builtFromFactCount: 3, builtAt: '2026-09-01T09:00:00Z' },
			facts: [{ factID: 'fact:sample', episodeID: 'episode:a', ownerPersonID: 'person:sample', circleIDs: [], kind: 'temporary', content: 'A remembered decision', validFrom: '2026-09-01T10:00:00Z', validUntil: '2026-09-03T10:00:00Z', reinforcementCount: 0, lastRecalledAt: '2026-09-02T10:00:00Z' }]
		});
		expect(response.profile.identityLines).toEqual(['Works on the platform team']);
		expect(response.profile.builtAt).toBe('2026-09-01T09:00:00Z');
		expect(response.facts[0]?.episodeID).toBe('episode:a');
		expect(response.facts[0]?.validFrom).toBe('2026-09-01T10:00:00Z');
		expect(response.facts[0]?.validUntil).toBe('2026-09-03T10:00:00Z');
		expect(response.facts[0]?.reinforcementCount).toBe(0);
		expect(response.facts[0]?.lastRecalledAt).toBe('2026-09-02T10:00:00Z');
	});

	test('targets exact remembered facts, carries the reason only when given, and never supplies a reader identity', () => {
		expect(factForgetRequest(['fact:sample'], '  ')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample'] } });
		expect(factForgetRequest(['fact:sample', 'fact:other'], 'moved teams')).toEqual({ capability: 'person.memory.facts.forget', path: '/memory/api/facts/forget', body: { factIDs: ['fact:sample', 'fact:other'], reason: 'moved teams' } });
	});
});
