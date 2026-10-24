import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'bun:test';
import { MessengerRelay } from './messenger-calls';
import { MessengerCopies } from './messenger-copies';
import { messengerStoreOf } from './messenger-store';
import { Transfers } from './transfers';
import type { RangedSource } from './transfer-store';

type Heard = { method: string; path: string; host: string | null; authorization: string | null; range: string | null; body: string };

const heard: Heard[] = [];
const blob = new TextEncoder().encode('the bytes of a picture');
const sha = new Bun.CryptoHasher('sha256').update(blob).digest('hex');
const video = Uint8Array.from({ length: 13 * 1024 * 1024 + 5 }, (_, index) => (index * 31 + 7) % 251);
const videoSha = digestOf(video);
const companyID = 'c1';
const host = 'acme.example.test';
const kept = new Map<string, Uint8Array<ArrayBuffer>>();
const resumableUploads = new Map<string, { path: string; length: number; parts: Uint8Array[] }>();
const requesterKey = 'b'.repeat(64);
const appAuthorization = `Nostr ${btoa(JSON.stringify({ kind: 24242, pubkey: requesterKey }))}`;
const readsTheRelayAllows = new Set(['Nostr get', appAuthorization]);
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
		if (!readsTheRelayAllows.has(request.headers.get('authorization') ?? '')) return new Response('auth required', { status: 401 });
		const headers = { 'content-type': 'image/png', 'cache-control': 'immutable' };
		if (request.method === 'HEAD') return new Response(null, { headers: { ...headers, 'content-length': String(blob.byteLength) } });
		return rangedAnswer(blob, request.headers.get('range'), headers);
	}
	if (url.pathname === '/upload') {
		if (request.headers.get('x-refuse') === 'yes') return new Response('refused', { status: 413 });
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
	const signingRead = after('/object/sign/asset/');
	if (signingRead !== null) {
		return kept.has(signingRead)
			? Response.json({ signedURL: `/object/sign/asset/${signingRead}?signature=r` })
			: Response.json({ statusCode: '404', error: 'not_found' }, { status: 400 });
	}
	if (path === '/object/move') return moved((await request.json()) as { sourceKey: string; destinationKey: string });
	if (path === '/upload/resumable') return resumableUploadOpened(request);
	const resumable = after('/upload/resumable/');
	if (resumable !== null) return resumablePartTaken(resumable, new Uint8Array(await request.arrayBuffer()));
	const signingUpload = after('/object/upload/sign/asset/');
	if (signingUpload !== null) return Response.json({ url: `/object/upload/sign/asset/${signingUpload}?token=u` });
	if (path === '/object/asset' && request.method === 'DELETE') {
		const { prefixes } = (await request.json()) as { prefixes: string[] };
		for (const prefix of prefixes) kept.delete(prefix);
		return Response.json([]);
	}
	return new Response('unexpected', { status: 599 });
}

function moved(asked: { sourceKey: string; destinationKey: string }): Response {
	const bytes = kept.get(asked.sourceKey);
	if (!bytes) return Response.json({ statusCode: '404' }, { status: 400 });
	if (kept.has(asked.destinationKey)) return Response.json({ statusCode: '409', message: 'The resource already exists' }, { status: 400 });
	kept.delete(asked.sourceKey);
	kept.set(asked.destinationKey, bytes);
	return Response.json({ message: 'Successfully moved' });
}

function resumableUploadOpened(request: Request): Response {
	const metadata = new Map(
		(request.headers.get('upload-metadata') ?? '').split(',').map((entry) => {
			const [name, value] = entry.split(' ');
			return [name, atob(value ?? '')];
		})
	);
	const path = metadata.get('objectName') ?? '';
	if (kept.has(path)) return Response.json({ statusCode: '409', error: 'Duplicate' }, { status: 409 });
	const id = crypto.randomUUID();
	resumableUploads.set(id, { path, length: Number(request.headers.get('upload-length')), parts: [] });
	return new Response(null, { status: 201, headers: { location: `/storage/v1/upload/resumable/${id}` } });
}

function resumablePartTaken(id: string, part: Uint8Array): Response {
	const upload = resumableUploads.get(id);
	if (!upload) return new Response('no such upload', { status: 404 });
	upload.parts.push(part);
	const whole = Buffer.concat(upload.parts);
	if (whole.byteLength >= upload.length) kept.set(upload.path, new Uint8Array(whole));
	return new Response(null, { status: 204 });
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
	resumableUploads.clear();
});

type Copying = { readersAsked: { requesterKey: string; mediaURL: string }[]; reported: string[] };

function messengerRelay(readsAs: () => RangedSource | null = readsAsTheApp): MessengerRelay & Copying {
	const access = { projectURL: `http://127.0.0.1:${storage.port}`, apiKey: 'publishable', accessToken: async () => 'host-token' };
	const copying: Copying = { readersAsked: [], reported: [] };
	const copies = new MessengerCopies({
		companyID,
		access,
		transfers: new Transfers(() => undefined),
		readerOf: async (key, mediaURL) => {
			copying.readersAsked.push({ requesterKey: key, mediaURL });
			return readsAs();
		},
		report: (line) => copying.reported.push(line)
	});
	return Object.assign(new MessengerRelay(`http://127.0.0.1:${relay.port}`, messengerStoreOf(companyID, access, copies)), copying);
}

function readsAsTheApp(): RangedSource {
	return (range) => fetch(`http://127.0.0.1:${relay.port}/media/${sha}.png`, { headers: { authorization: 'Nostr get', range, host } });
}

function copyPathOf(digest: string): string {
	return `${companyID}/shared/transfer/${digest}`;
}

async function until(condition: () => boolean, what: string): Promise<void> {
	for (let round = 0; round < 400; round += 1) {
		if (condition()) return;
		await Bun.sleep(5);
	}
	throw new Error(`waited for ${what}`);
}

function headAsTheApp(messenger: MessengerRelay, authorization = appAuthorization): ReturnType<MessengerRelay['serve']> {
	return messenger.serve('messenger.media.head', { method: 'HEAD', path: `/media/${sha}.png`, host, headers: { authorization } });
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

	test('stages an upload, streams the staged bytes to the relay, and keeps them as the copy of what it accepted', async () => {
		const messenger = messengerRelay();
		const stage = await messenger.serve('messenger.media.stage', { method: 'PUT', path: '/upload', host });
		const { stagedPath, uploadURL } = stage.body as { stagedPath: string; uploadURL: string };
		expect(stagedPath.startsWith(`${companyID}/shared/transfer/staged-`)).toBe(true);
		expect(uploadURL).toBe(storeAddress(`/object/upload/sign/asset/${stagedPath}?token=u`));
		const uploadedBytes = new TextEncoder().encode('uploaded bytes');
		kept.set(stagedPath, uploadedBytes);

		const written = await messenger.serve('messenger.media.write', {
			method: 'PUT',
			path: '/upload',
			host,
			headers: { authorization: 'Nostr upload', 'x-sha-256': sha },
			stagedPath
		});
		expect(heard.at(-1)).toMatchObject({ method: 'PUT', path: '/upload', host, authorization: 'Nostr upload', body: 'uploaded bytes' });
		expect(written.status).toBe(200);
		const uploaded = digestOf(uploadedBytes);
		expect(JSON.parse(atob((written.body as { bodyBase64: string }).bodyBase64))).toEqual({
			sha256: uploaded,
			size: 14,
			url: `https://${host}/media/${uploaded}`
		});
		expect(kept.has(stagedPath)).toBe(false);
		expect(kept.get(copyPathOf(uploaded))).toEqual(uploadedBytes);
	});

	test('lets a staged upload go when the relay refuses it, keeping no copy', async () => {
		const messenger = messengerRelay();
		const stage = await messenger.serve('messenger.media.stage', { method: 'PUT', path: '/upload', host });
		const { stagedPath } = stage.body as { stagedPath: string };
		kept.set(stagedPath, new TextEncoder().encode('too much'));
		const written = await messenger.serve('messenger.media.write', {
			method: 'PUT',
			path: '/upload',
			host,
			headers: { authorization: 'Nostr upload', 'x-refuse': 'yes' },
			stagedPath
		});
		expect(written.status).toBe(413);
		expect(kept.size).toBe(0);
	});

	test('lets the staged upload go when a copy of the same bytes is already kept', async () => {
		const messenger = messengerRelay();
		const stage = await messenger.serve('messenger.media.stage', { method: 'PUT', path: '/upload', host });
		const { stagedPath } = stage.body as { stagedPath: string };
		const uploadedBytes = new TextEncoder().encode('uploaded twice');
		kept.set(stagedPath, uploadedBytes);
		kept.set(copyPathOf(digestOf(uploadedBytes)), uploadedBytes);
		const written = await messenger.serve('messenger.media.write', { method: 'PUT', path: '/upload', host, headers: {}, stagedPath });
		expect(written.status).toBe(200);
		expect([...kept.keys()]).toEqual([copyPathOf(digestOf(uploadedBytes))]);
		expect(messenger.reported).toEqual([]);
	});

	test('answers a head the relay allows with the address of the copy the company keeps, filling nothing', async () => {
		kept.set(copyPathOf(sha), blob);
		const messenger = messengerRelay();
		const answer = await headAsTheApp(messenger);
		expect(heard.at(-1)).toMatchObject({ method: 'HEAD', path: `/media/${sha}.png`, host, authorization: appAuthorization });
		expect(answer.status).toBe(200);
		expect(answer.body).toMatchObject({
			headers: { 'content-type': 'image/png', 'content-length': String(blob.byteLength) },
			bodyBase64: '',
			objectURL: storeAddress(`/object/sign/asset/${copyPathOf(sha)}?signature=r`)
		});
		expect(messenger.readersAsked).toEqual([]);
	});

	test('answers a head the relay refuses with the refusal alone, neither signing the copy nor filling one', async () => {
		kept.set(copyPathOf(sha), blob);
		const messenger = messengerRelay();
		const answer = await headAsTheApp(messenger, 'Nostr stolen');
		expect(answer.status).toBe(401);
		expect(answer.body).not.toHaveProperty('objectURL');
		await Bun.sleep(20);
		expect(messenger.readersAsked).toEqual([]);
		expect(resumableUploads.size).toBe(0);
	});

	test('fills a missing copy once, reading as the requester, however many heads ask for it at once', async () => {
		const messenger = messengerRelay();
		const answers = await Promise.all(Array.from({ length: 5 }, () => headAsTheApp(messenger)));
		for (const answer of answers) {
			expect(answer.status).toBe(200);
			expect(answer.body).not.toHaveProperty('objectURL');
		}
		await until(() => kept.has(copyPathOf(sha)), 'the copy to be filled');
		expect(kept.get(copyPathOf(sha))).toEqual(blob);
		expect(resumableUploads.size).toBe(1);
		expect(messenger.readersAsked).toEqual([{ requesterKey, mediaURL: `https://${host}/media/${sha}.png` }]);

		const later = await headAsTheApp(messenger);
		expect(later.body).toHaveProperty('objectURL');
	});

	test('logs one line naming the digest when the company holds no key to fill a copy with', async () => {
		const messenger = messengerRelay(() => null);
		await headAsTheApp(messenger);
		await until(() => messenger.reported.length > 0, 'the failure to be logged');
		await Bun.sleep(20);
		expect(messenger.reported).toEqual([`messenger copy ${sha} not filled: the company holds no messenger key for ${requesterKey}`]);
		expect(kept.size).toBe(0);
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
