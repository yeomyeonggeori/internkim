import { describe, expect, test } from 'bun:test';
import { fetchMemoryFacts, forgetMemoryFact, normalizeMemoryFactsResponse } from '../../../src/routes/memory/memory-facts-api';

describe('memory facts api normalizer', () => {
	test('keeps well-formed facts and drops the rest', () => {
		const response = normalizeMemoryFactsResponse({
			personID: 'person-1',
			embeddingModel: 'perplexity/pplx-embed-v1-4b',
			profile: { identityLines: ['이샘플 prefers bullets', 7], currentLines: [], builtAt: '2026-09-02T10:00:00Z' },
			facts: [
				{
					factID: 'fact-1',
					episodeID: 'episode-1',
					ownerPersonID: 'person-1',
					circleIDs: ['member', 7],
					kind: 'preference',
					content: '이샘플 prefers bullets',
					validFrom: '2026-09-02T10:00:00Z',
					validUntil: '0001-01-01T00:00:00Z',
					reinforcementCount: 2,
					lastRecalledAt: '0001-01-01T00:00:00Z'
				},
				{ factID: 'fact-2', content: 'missing kind and scope', validFrom: '2026-09-02T10:00:00Z' },
				{ factID: 'fact-3', kind: 'rumour', ownerPersonID: 'person-1', content: 'unknown kind', validFrom: '2026-09-02T10:00:00Z' },
				'not a fact'
			]
		});

		expect(response.personID).toBe('person-1');
		expect(response.embeddingModel).toBe('perplexity/pplx-embed-v1-4b');
		expect(response.profile.identityLines).toEqual(['이샘플 prefers bullets']);
		expect(response.profile.builtAt).toBe('2026-09-02T10:00:00Z');
		expect(response.facts.length).toBe(1);
		expect(response.facts[0]).toEqual({
			factID: 'fact-1',
			episodeID: 'episode-1',
			ownerPersonID: 'person-1',
			circleIDs: ['member'],
			kind: 'preference',
			content: '이샘플 prefers bullets',
			validFrom: '2026-09-02T10:00:00Z',
			reinforcementCount: 2
		});
	});

	test('tolerates an empty document', () => {
		const response = normalizeMemoryFactsResponse(null);
		expect(response).toEqual({ personID: '', profile: { identityLines: [], currentLines: [] }, facts: [] });
	});

	test('does not expose fetch failure response text', async () => {
		const originalFetch = globalThis.fetch;
		globalThis.fetch = createFetchStub(async () => new Response('Traceback /workspace/.blueclaw/private.go', { status: 502 }));
		try {
			const errorMessage = await rejectedErrorMessage(fetchMemoryFacts());
			expect(errorMessage).toBe('Memory facts request returned 502');
			expect(errorMessage.includes('Traceback')).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('posts a forget request with the fact id and reason', async () => {
		const originalFetch = globalThis.fetch;
		const requests: Array<{ input: Parameters<typeof fetch>[0]; initialization?: Parameters<typeof fetch>[1] }> = [];
		globalThis.fetch = createFetchStub(async (input, initialization) => {
			requests.push({ input, initialization });
			return new Response('{"forgottenFactIDs":["fact-1"]}', { status: 200 });
		});
		try {
			await forgetMemoryFact('fact-1', 'asked in the web app');
			expect(requests.map((request) => request.input)).toEqual(['/memory/api/facts/forget']);
			expect(requests[0]?.initialization?.method).toBe('POST');
			expect(requests[0]?.initialization?.credentials).toBe('include');
			expect(JSON.parse(String(requests[0]?.initialization?.body))).toEqual({ factIDs: ['fact-1'], reason: 'asked in the web app' });
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});

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
