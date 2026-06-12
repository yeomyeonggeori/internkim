import { describe, expect, test } from 'bun:test';
import { fetchMemoryGraph, normalizeMemoryGraphResponse } from '../../../src/routes/memory/memory-graph-api';

describe('memory graph api normalizer', () => {
	test('drops malformed graph rows while keeping valid graph data', () => {
		const response = normalizeMemoryGraphResponse({
			health: {
				configured: true,
				reachable: false,
				error: 'Traceback private graph detail',
				lastIngestionError: '/workspace/.blueclaw/private.py failed',
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
		expect(response.health?.hasGraphFailure).toBe(true);
		expect(response.health?.hasIngestionFailure).toBe(true);
		expect(response.health?.hasSearchFailure).toBe(undefined);
		expect(hasProperty(response.health, 'error')).toBe(false);
		expect(hasProperty(response.health, 'lastIngestionError')).toBe(false);
		expect(hasProperty(response.health, 'lastSearchError')).toBe(false);
		expect(response.namespaces?.length).toBe(1);
		expect(response.episodes?.length).toBe(2);
		expect(response.facts?.length).toBe(1);
		expect(response.nodes?.length).toBe(1);
		expect(response.edges?.length).toBe(1);
		expect(response.edges?.[0]?.weight).toBe(2);
	});

	test('normalizes boolean failure flags without preserving raw detail strings', () => {
		const response = normalizeMemoryGraphResponse({
			health: {
				configured: true,
				reachable: true,
				error: false,
				lastSearchError: true,
				lastIngestionError: ''
			}
		});

		expect(response.health?.hasGraphFailure).toBe(undefined);
		expect(response.health?.hasSearchFailure).toBe(true);
		expect(response.health?.hasIngestionFailure).toBe(undefined);
		expect(hasProperty(response.health, 'lastSearchError')).toBe(false);
	});

	test('does not expose graph fetch failure response text', async () => {
		const originalFetch = globalThis.fetch;
		const fetchStub = createFetchStub(async () => new Response('Traceback /workspace/.blueclaw/private.py', { status: 502 }));
		globalThis.fetch = fetchStub;

		try {
			const errorMessage = await rejectedErrorMessage(fetchMemoryGraph('team'));

			expect(errorMessage).toBe('Memory graph request returned 502');
			expect(errorMessage.includes('Traceback')).toBe(false);
			expect(errorMessage.includes('/workspace')).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});

function hasProperty(document: object | undefined, propertyName: string): boolean {
	return document ? Object.hasOwn(document, propertyName) : false;
}

function createFetchStub(
	handler: (input: Parameters<typeof fetch>[0], initialization?: Parameters<typeof fetch>[1]) => Promise<Response>
) {
	return Object.assign(handler, { preconnect: globalThis.fetch.preconnect });
}

async function rejectedErrorMessage(promise: Promise<unknown>): Promise<string> {
	try {
		await promise;
		return '';
	} catch (error) {
		return error instanceof Error ? error.message : '';
	}
}
