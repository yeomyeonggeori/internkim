import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';
import { MessengerRelay } from './messenger-calls';
import { messengerStoreOf } from './messenger-store';

type Heard = { method: string; path: string; host: string | null; authorization: string | null; range: string | null; body: string };

const heard: Heard[] = [];
const blob = new TextEncoder().encode('the bytes of a picture');
const sha = new Bun.CryptoHasher('sha256').update(blob).digest('hex');
const video = Uint8Array.from({ length: 13 * 1024 * 1024 + 5 }, (_, index) => (index * 31 + 7) % 251);
const videoSha = digestOf(video);
const companyID = 'c1';
const host = 'acme.example.test';
const kept = new Map<string, Uint8Array<ArrayBuffer>>();
let relay: ReturnType<typeof Bun.serve>;
let storage: ReturnType<typeof Bun.serve>;

function rangedAnswer(bytes: Uint8Array<ArrayBuffer>, range: string | null, extra: Record<string, string> = {}): Response {
	const matched = /^bytes=(\d+)-(\d+)$/.exec(range ?? '');
	if (!matched) return new Response(bytes, { headers: extra });
	const first = Number(matched[1]);
	if (first >= bytes.byteLength) return new Response(null, { status: 416 });
	const last = Math.min(Number(matched[2]), bytes.byteLength - 1);
	return new Response(bytes.slice(first, last + 1), {
		status: 206,
		headers: { ...extra, 'content-range': `bytes ${first}-${last}/${bytes.byteLength}` }
	});
}

function digestOf(bytes: Uint8Array): string {
	return new Bun.CryptoHasher('sha256').update(bytes).digest('hex');
}

function servedByTheRelay(request: Request, url: URL, received: Uint8Array): Response {
	if (url.pathname === '/query') return Response.json({ events: [] }, { headers: { 'x-relay': 'buzz' } });
	if (url.pathname === `/media/${sha}.png`) {
		if (request.headers.get('authorization') !== 'Nostr get') return new Response('auth required', { status: 401 });
		const headers = { 'content-type': 'image/png', 'cache-control': 'immutable' };
		if (request.method === 'HEAD') return new Response(null, { headers: { ...headers, 'content-length': String(blob.byteLength) } });
		return rangedAnswer(blob, request.headers.get('range'), headers);
	}
	if (url.pathname === '/upload') {
		const uploaded = digestOf(received);
		return Response.json({ sha256: uploaded, size: received.byteLength, url: `https://${request.headers.get('host')}/media/${uploaded}` });
	}
	return new Response('not found', { status: 404 });
}

async function servedByTheStore(request: Request, url: URL): Promise<Response> {
	const path = url.pathname.replace(/^\/storage\/v1/, '');
	const after = (prefix: string) => (path.startsWith(prefix) ? decodeURIComponent(path.slice(prefix.length)) : null);
	const informed = after('/object/info/authenticated/asset/');
	if (informed !== null) {
		const bytes = kept.get(informed);
		return bytes ? Response.json({ size: bytes.byteLength }) : Response.json({ statusCode: '404' }, { status: 400 });
	}
	const read = after('/object/authenticated/asset/');
	if (read !== null) {
		const bytes = kept.get(read);
		return bytes ? rangedAnswer(bytes, request.headers.get('range')) : new Response('missing', { status: 404 });
	}
	const signingUpload = after('/object/upload/sign/asset/');
	if (signingUpload !== null) return Response.json({ url: `/object/upload/sign/asset/${signingUpload}?token=u` });
	if (path === '/object/asset' && request.method === 'DELETE') {
		const { prefixes } = (await request.json()) as { prefixes: string[] };
		for (const prefix of prefixes) kept.delete(prefix);
		return Response.json([]);
	}
	return new Response('unexpected', { status: 599 });
}

beforeAll(() => {
	relay = Bun.serve({
		hostname: '127.0.0.1',
		port: 0,
		async fetch(request) {
			const url = new URL(request.url);
			const received = new Uint8Array(await request.arrayBuffer());
			heard.push({
				method: request.method,
				path: `${url.pathname}${url.search}`,
				host: request.headers.get('host'),
				authorization: request.headers.get('authorization'),
				range: request.headers.get('range'),
				body: received.byteLength > 1024 ? `${received.byteLength} bytes` : new TextDecoder().decode(received)
			});
			return servedByTheRelay(request, url, received);
		}
	});
	storage = Bun.serve({ hostname: '127.0.0.1', port: 0, fetch: (request) => servedByTheStore(request, new URL(request.url)) });
});

afterAll(() => {
	relay.stop(true);
	storage.stop(true);
});

beforeEach(() => {
	kept.clear();
});

function messengerRelay(): MessengerRelay {
	const access = { projectURL: `http://127.0.0.1:${storage.port}`, apiKey: 'publishable', accessToken: async () => 'host-token' };
	return new MessengerRelay(`http://127.0.0.1:${relay.port}`, messengerStoreOf(companyID, access));
}

function storeAddress(path: string): string {
	return `http://127.0.0.1:${storage.port}/storage/v1${path}`;
}

describe('MessengerRelay', () => {
	test('carries an http request to the relay under the address the app dialled', async () => {
		const answer = await messengerRelay().serve('messenger.http', {
			method: 'POST',
			path: '/query?limit=1',
			host,
			headers: { authorization: 'Nostr nip98', 'content-type': 'application/json' },
			bodyBase64: btoa('{"kinds":[1]}')
		});
		expect(heard.at(-1)).toEqual({
			method: 'POST',
			path: '/query?limit=1',
			host,
			authorization: 'Nostr nip98',
			range: null,
			body: '{"kinds":[1]}'
		});
		expect(answer.status).toBe(200);
		expect(answer.body).toMatchObject({ headers: { 'x-relay': 'buzz' }, bodyBase64: btoa('{"events":[]}') });
	});

	test('answers a media read from a gateway that does not stream by carrying the relay answer inline', async () => {
		const answer = await messengerRelay().serve('messenger.media.read', {
			method: 'GET',
			path: `/media/${sha}.png`,
			host,
			headers: { authorization: 'Nostr get', range: 'bytes=4-8' }
		});
		expect(heard.at(-1)).toMatchObject({ method: 'GET', host, authorization: 'Nostr get', range: 'bytes=4-8' });
		expect(answer.status).toBe(206);
		expect(answer.body).toMatchObject({
			headers: { 'content-range': `bytes 4-8/${blob.byteLength}`, 'content-type': 'image/png' },
			bodyBase64: btoa('bytes')
		});
		expect(answer.body).not.toHaveProperty('objectURL');
		expect(kept.size).toBe(0);
	});

	test('answers a HEAD from the relay alone', async () => {
		const answer = await messengerRelay().serve('messenger.media.read', {
			method: 'HEAD',
			path: `/media/${sha}.png`,
			host,
			headers: { authorization: 'Nostr get' }
		});
		expect(answer.status).toBe(200);
		expect(answer.body).toMatchObject({ headers: { 'content-length': String(blob.byteLength) }, bodyBase64: '' });
		expect(kept.size).toBe(0);
	});

	test('stages an upload, streams the staged bytes to the relay, and lets them go', async () => {
		const messenger = messengerRelay();
		const stage = await messenger.serve('messenger.media.stage', { method: 'PUT', path: '/upload', host });
		const { stagedPath, uploadURL } = stage.body as { stagedPath: string; uploadURL: string };
		expect(stagedPath.startsWith(`${companyID}/shared/transfer/staged-`)).toBe(true);
		expect(uploadURL).toBe(storeAddress(`/object/upload/sign/asset/${stagedPath}?token=u`));
		kept.set(stagedPath, new TextEncoder().encode('uploaded bytes'));

		const written = await messenger.serve('messenger.media.write', {
			method: 'PUT',
			path: '/upload',
			host,
			headers: { authorization: 'Nostr upload', 'x-sha-256': sha },
			stagedPath
		});
		expect(heard.at(-1)).toMatchObject({ method: 'PUT', path: '/upload', host, authorization: 'Nostr upload', body: 'uploaded bytes' });
		expect(written.status).toBe(200);
		const uploaded = digestOf(new TextEncoder().encode('uploaded bytes'));
		expect(JSON.parse(atob((written.body as { bodyBase64: string }).bodyBase64))).toEqual({
			sha256: uploaded,
			size: 14,
			url: `https://${host}/media/${uploaded}`
		});
		expect(kept.has(stagedPath)).toBe(false);
	});

	test('streams a staged upload larger than two transfer chunks to the relay intact', async () => {
		const messenger = messengerRelay();
		const stage = await messenger.serve('messenger.media.stage', { method: 'PUT', path: '/upload', host });
		const { stagedPath } = stage.body as { stagedPath: string };
		kept.set(stagedPath, video);
		const written = await messenger.serve('messenger.media.write', {
			method: 'PUT',
			path: '/upload',
			host,
			headers: { authorization: 'Nostr upload', 'x-sha-256': videoSha },
			stagedPath
		});
		expect(written.status).toBe(200);
		const descriptor = JSON.parse(atob((written.body as { bodyBase64: string }).bodyBase64));
		expect(descriptor).toMatchObject({ sha256: videoSha, size: video.byteLength });
	});

	test('takes an upload only from where uploads are staged', async () => {
		const answer = await messengerRelay().serve('messenger.media.write', {
			method: 'PUT',
			path: '/upload',
			host,
			stagedPath: `${companyID}/shared/attachment/someone-elses`
		});
		expect(answer.status).toBe(400);
	});
});
