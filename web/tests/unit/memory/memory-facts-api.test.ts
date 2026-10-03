import { describe, expect, test } from 'bun:test';
import { fetchMemoryFacts, forgetMemoryFact } from '../../../src/routes/memory/memory-facts-api';

describe('the memory facts api', () => {
	test('refuses an answer that is not the shape blueclaw gives', async () => {
		const originalFetch = globalThis.fetch;
		globalThis.fetch = createFetchStub(async () => Response.json({ personID: 'person-1', facts: [{ factID: 'fact-1', kind: 'preference' }] }));
		try {
			expect(await rejectedErrorMessage(fetchMemoryFacts())).not.toBe('');
		} finally {
			globalThis.fetch = originalFetch;
		}
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

	test('does not expose forget failure response text', async () => {
		const originalFetch = globalThis.fetch;
		globalThis.fetch = createFetchStub(async () => new Response('none of the facts are live and readable by this person', { status: 404 }));
		try {
			expect(await rejectedErrorMessage(forgetMemoryFact('fact-1', ''))).toBe('Memory forget request returned 404');
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
