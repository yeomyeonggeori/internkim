import { afterEach, describe, expect, test } from 'bun:test';
import { uploadChunkBytes, uploadToStore } from '$lib/transfer/store-upload';

const realFetch = globalThis.fetch;
const session = {
	projectURL: 'https://company.supabase.test',
	publishableKey: 'publishable',
	accessToken: async () => 'member-token',
	companyID: 'company-1',
	memberID: 'member-1'
};

afterEach(() => {
	globalThis.fetch = realFetch;
});

type Seen = { method: string; offset: string | null; sizeBytes: number; authorization: string | null; metadata: string | null };

function storeThatDropsOnce(dropAtOffset: number): Seen[] {
	const seen: Seen[] = [];
	let storedBytes = 0;
	let hasDropped = false;
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const headers = new Headers(init?.headers);
		const method = init?.method ?? 'GET';
		const body = init?.body instanceof Blob ? init.body : null;
		seen.push({
			method,
			offset: headers.get('upload-offset'),
			sizeBytes: body?.size ?? 0,
			authorization: headers.get('authorization'),
			metadata: headers.get('upload-metadata')
		});
		if (method === 'POST') return new Response(null, { status: 201, headers: { location: '/storage/v1/upload/resumable/abc' } });
		if (method === 'HEAD') return new Response(null, { status: 200, headers: { 'upload-offset': String(storedBytes) } });
		if (!hasDropped && Number(headers.get('upload-offset')) === dropAtOffset) {
			hasDropped = true;
			throw new TypeError('network connection was lost');
		}
		storedBytes = Number(headers.get('upload-offset')) + (body?.size ?? 0);
		expect(String(input)).toBe('https://company.supabase.test/storage/v1/upload/resumable/abc');
		return new Response(null, { status: 204, headers: { 'upload-offset': String(storedBytes) } });
	}) as typeof fetch;
	return seen;
}

describe('a file put in the store from the browser', () => {
	test('goes up in chunks the store accepts, picks up where a dropped one left off, and lands under the member', async () => {
		const seen = storeThatDropsOnce(uploadChunkBytes);
		const file = new Blob([new Uint8Array(2 * uploadChunkBytes + 1000)]);
		const progress: number[] = [];

		const object = await uploadToStore(session, file, 'video/mp4', (sent) => progress.push(sent));

		expect(object.startsWith('company-1/person/member-1/upload/')).toBe(true);
		const patches = seen.filter((one) => one.method === 'PATCH');
		expect(patches.map((one) => [one.offset, one.sizeBytes])).toEqual([
			['0', uploadChunkBytes],
			[String(uploadChunkBytes), uploadChunkBytes],
			[String(uploadChunkBytes), uploadChunkBytes],
			[String(2 * uploadChunkBytes), 1000]
		]);
		expect(seen.some((one) => one.method === 'HEAD')).toBe(true);
		expect(progress).toEqual([uploadChunkBytes, 2 * uploadChunkBytes, 2 * uploadChunkBytes + 1000]);
		expect(seen.every((one) => one.authorization === 'Bearer member-token')).toBe(true);
		expect(atob(String(seen[0]?.metadata?.split(',')[1]?.split(' ')[1]))).toBe(object);
	});

	test('a chunk the store refuses is reported with the store\'s own reason and not sent again', async () => {
		const methods: string[] = [];
		globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			methods.push(method);
			if (method === 'POST') return new Response(null, { status: 201, headers: { location: '/storage/v1/upload/resumable/abc' } });
			return Response.json({ message: 'new row violates row-level security policy' }, { status: 403 });
		}) as typeof fetch;

		const failure = await uploadToStore(session, new Blob([new Uint8Array(10)]), 'video/mp4').catch((thrown: unknown) => thrown);

		expect(failure).toMatchObject({ status: 403 });
		expect(String(failure)).toContain('row-level security');
		expect(methods).toEqual(['POST', 'PATCH']);
	});
});
