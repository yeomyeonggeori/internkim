import { describe, expect, test } from 'bun:test';
import {
	deleteMemoryEpisode,
	fetchMemoryGraph,
	normalizeMemoryGraphResponse,
	savePinnedMemory
} from '../../../src/routes/memory/memory-graph-api';

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
			episodes: [
				{ episodeID: 'episode-1', prompt: 'Remember this', namespaceIDs: ['person-local/private', 12] },
				'episode-2'
			],
			facts: [
				{
					factID: 'fact-1',
					scopeType: 'person',
					namespaceID: 'person-local/private',
					content: 'Memory fact',
					score: 0.91,
					sourceEpisodeID: 'episode-1'
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
		expect(response.episodes?.length).toBe(1);
		expect(response.episodes?.[0]?.episodeID).toBe('episode-1');
		expect(response.episodes?.[0]?.prompt).toBe('Remember this');
		expect(response.episodes?.[0]?.namespaceIDs).toEqual(['person-local/private']);
		expect(response.facts?.length).toBe(1);
		expect(response.facts?.[0]?.sourceEpisodeID).toBe('episode-1');
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

	test('posts memory mutations with JSON bodies', async () => {
		const originalFetch = globalThis.fetch;
		const requests: Array<{ input: Parameters<typeof fetch>[0]; initialization?: Parameters<typeof fetch>[1] }> = [];
		const fetchStub = createFetchStub(async (input, initialization) => {
			requests.push({ input, initialization });
			return new Response('{}', { status: 200 });
		});
		globalThis.fetch = fetchStub;

		try {
			await deleteMemoryEpisode('episode-1', ['user:person-1']);
			await savePinnedMemory('# Memory\n- New memory.');

			expect(requests.map((request) => request.input)).toEqual(['/memory/api/episodes/delete', '/memory/api/pinned/update']);
			expect(requests[0]?.initialization?.method).toBe('POST');
			expect(requests[0]?.initialization?.credentials).toBe('include');
			expect(JSON.parse(String(requests[0]?.initialization?.body))).toEqual({
				episodeID: 'episode-1',
				namespaceIDs: ['user:person-1']
			});
			expect(JSON.parse(String(requests[1]?.initialization?.body))).toEqual({
				content: '# Memory\n- New memory.'
			});
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
	return Object.assign(handler, { preconnect: (url: string | URL): void => void url });
}

async function rejectedErrorMessage(promise: Promise<unknown>): Promise<string> {
	try {
		await promise;
		return '';
	} catch (error) {
		return error instanceof Error ? error.message : '';
	}
}
