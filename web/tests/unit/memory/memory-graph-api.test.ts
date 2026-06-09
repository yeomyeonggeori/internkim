import { describe, expect, test } from 'bun:test';
import { normalizeMemoryGraphResponse } from '../../../src/routes/memory/memory-graph-api';

describe('memory graph api normalizer', () => {
	test('drops malformed graph rows while keeping valid graph data', () => {
		const response = normalizeMemoryGraphResponse({
			health: {
				configured: true,
				reachable: false,
				error: 'graph unavailable',
				lastSearchError: 123
			},
			namespaces: [
				{ namespaceID: 'person-local/private', scopeType: 'person', episodeCount: 2 },
				{ namespaceID: 123, scopeType: 'person' }
			],
			episodes: [{ episodeID: 'episode-1' }, 'episode-2'],
			facts: [
				{
					factID: 'fact-1',
					scopeType: 'person',
					namespaceID: 'person-local/private',
					content: 'Memory fact',
					score: 0.91
				},
				{ factID: 'fact-2', content: 'missing required fields' }
			],
			nodes: [
				{ nodeID: 'node-1', label: 'Memory', kind: 'fact', scopeType: 'person' },
				{ nodeID: 'node-2', kind: 'fact' }
			],
			edges: [
				{ sourceID: 'node-1', targetID: 'node-2', weight: 2 },
				{ sourceID: 'node-1', targetID: 2 }
			]
		});

		expect(response.health?.configured).toBe(true);
		expect(response.health?.reachable).toBe(false);
		expect(response.health?.error).toBe('graph unavailable');
		expect(response.health?.lastSearchError).toBe(undefined);
		expect(response.namespaces?.length).toBe(1);
		expect(response.episodes?.length).toBe(2);
		expect(response.facts?.length).toBe(1);
		expect(response.nodes?.length).toBe(1);
		expect(response.edges?.length).toBe(1);
		expect(response.edges?.[0]?.weight).toBe(2);
	});
});
