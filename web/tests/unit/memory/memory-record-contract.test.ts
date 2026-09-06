import { describe, expect, test } from 'bun:test';
import { factDeleteRequest, factUpdateRequest, normalizeMemoryGraphResponse } from '../../../src/routes/memory/memory-graph-api';

describe('memory record evidence', () => {
	test('preserves source provenance, validity and partial retrieval independently', () => {
		const response = normalizeMemoryGraphResponse({
			facts: [{ factID: 'fact:sample', namespaceID: 'user:sample', scopeType: 'user', content: 'A remembered decision', sourceEpisodeIDs: ['episode:a', 'episode:b'], recordedAt: '2026-09-01T10:00:00Z', invalidAt: '2026-09-03T10:00:00Z', validAt: '0001-01-01T00:00:00Z' }],
			retrieval: { query: 'What did we decide?', complete: false, limit: 120, failures: [{ namespaceID: 'user:sample', message: 'Search unavailable' }] }
		});
		expect(response.facts?.[0]?.sourceEpisodeIDs).toEqual(['episode:a', 'episode:b']);
		expect(response.facts?.[0]?.validAt).toBeUndefined();
		expect(response.facts?.[0]?.recordedAt).toBe('2026-09-01T10:00:00Z');
		expect(response.facts?.[0]?.invalidAt).toBe('2026-09-03T10:00:00Z');
		expect(response.retrieval?.complete).toBe(false);
		expect(response.retrieval?.failures).toHaveLength(1);
	});

	test('targets one exact remembered fact and never supplies a reader identity', () => {
		expect(factUpdateRequest('fact:sample', 'user:sample', 'Corrected decision')).toEqual({ capability: 'person.memory.fact_update', path: '/memory/api/facts/update', body: { factID: 'fact:sample', namespaceID: 'user:sample', content: 'Corrected decision' } });
		expect(factDeleteRequest('fact:sample', 'user:sample')).toEqual({ capability: 'person.memory.fact_delete', path: '/memory/api/facts/delete', body: { factID: 'fact:sample', namespaceID: 'user:sample' } });
	});
});
