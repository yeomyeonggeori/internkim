import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { FairOutbox } from './fair-outbox';
import { MessengerMediaReads } from './messenger-media';
import { bytesOfBase64 } from '../../workers/connection-gateway/src/base64';
import { largestStreamFrameBytes, mediaCreditWindowBytes } from '../../workers/connection-gateway/src/host-protocol';

type Heard = { method: string; path: string; host: string | null; authorization: string | null; range: string | null };
type Sent = { kind: string; streamID: string; status?: number; headers?: Record<string, string>; data?: string; reason?: string };

const host = 'acme.example.test';
const video = Uint8Array.from({ length: 3 * 1024 * 1024 + 7 }, (_, index) => (index * 31 + 7) % 251);
const heard: Heard[] = [];
const endlessAborted: string[] = [];
let relay: ReturnType<typeof Bun.serve>;

function rangedAnswer(bytes: Uint8Array<ArrayBuffer>, range: string | null): Response {
	const matched = /^bytes=(\d+)-(\d+)$/.exec(range ?? '');
	const headers = { 'content-type': 'video/mp4', 'accept-ranges': 'bytes' };
	if (!matched) return new Response(bytes, { headers });
	const first = Number(matched[1]);
	const last = Math.min(Number(matched[2]), bytes.byteLength - 1);
	return new Response(bytes.slice(first, last + 1), {
		status: 206,
		headers: { ...headers, 'content-range': `bytes ${first}-${last}/${bytes.byteLength}` }
	});
}

function endlessAnswer(request: Request): Response {
	request.signal.addEventListener('abort', () => endlessAborted.push(new URL(request.url).pathname));
	const chunk = new Uint8Array(64 * 1024);
	const body = new ReadableStream({
		async pull(controller) {
			await Bun.sleep(1);
			controller.enqueue(chunk);
		}
	});
	return new Response(body);
}

beforeAll(() => {
	relay = Bun.serve({
		hostname: '127.0.0.1',
		port: 0,
		fetch(request) {
			const url = new URL(request.url);
			heard.push({
				method: request.method,
				path: url.pathname,
				host: request.headers.get('host'),
				authorization: request.headers.get('authorization'),
				range: request.headers.get('range')
			});
			if (request.headers.get('authorization') !== 'Nostr get') return new Response('auth required', { status: 401 });
			if (url.pathname === '/media/video.mp4') return rangedAnswer(video, request.headers.get('range'));
			if (url.pathname.startsWith('/media/endless')) return endlessAnswer(request);
			return new Response('not found', { status: 404 });
		}
	});
});

afterAll(() => relay.stop(true));

function mediaReadsOver(relayURL: string): { reads: MessengerMediaReads; sent: Sent[]; reported: string[] } {
	const sent: Sent[] = [];
	const reported: string[] = [];
	const outbox = new FairOutbox(() => ({ send: (document) => sent.push(JSON.parse(document)), bufferedBytes: () => 0 }));
	return { reads: new MessengerMediaReads(relayURL, outbox, (line) => reported.push(line)), sent, reported };
}

function relayed(): { reads: MessengerMediaReads; sent: Sent[]; reported: string[] } {
	return mediaReadsOver(`http://127.0.0.1:${relay.port}`);
}

function open(reads: MessengerMediaReads, streamID: string, path: string, headers: Record<string, string>, method = 'GET'): void {
	reads.take({ kind: 'media.open', streamID, method, path, host, headers, creditBytes: mediaCreditWindowBytes });
}

async function until(condition: () => boolean, what: string): Promise<void> {
	for (let round = 0; round < 1000; round += 1) {
		if (condition()) return;
		await Bun.sleep(5);
	}
	throw new Error(`waited for ${what}`);
}

function bodyBytesOf(sent: Sent[]): number {
	return sent.filter((one) => one.kind === 'media.body').reduce((total, one) => total + bytesOfBase64(one.data ?? '').byteLength, 0);
}

async function readEverythingGrantingCredit(reads: MessengerMediaReads, sent: Sent[], streamID: string): Promise<Uint8Array> {
	let granted = 0;
	await until(() => {
		const arrived = bodyBytesOf(sent);
		if (arrived > granted) {
			reads.take({ kind: 'media.credit', streamID, bytes: arrived - granted });
			granted = arrived;
		}
		return sent.some((one) => one.kind === 'media.end' || one.kind === 'media.fail');
	}, 'the read to end');
	return Buffer.concat(sent.filter((one) => one.kind === 'media.body').map((one) => bytesOfBase64(one.data ?? '')));
}

describe('MessengerMediaReads', () => {
	test('streams a blob several frames long to the gateway intact and in order', async () => {
		const { reads, sent } = relayed();
		open(reads, 's1', '/media/video.mp4', { authorization: 'Nostr get' });
		const body = await readEverythingGrantingCredit(reads, sent, 's1');
		expect(sent[0]).toMatchObject({ kind: 'media.head', streamID: 's1', status: 200, headers: { 'content-length': String(video.byteLength) } });
		expect(sent.at(-1)).toEqual({ kind: 'media.end', streamID: 's1' });
		expect(sent.filter((one) => one.kind === 'media.body').length).toBeGreaterThan(video.byteLength / largestStreamFrameBytes);
		expect(Buffer.compare(body, video)).toBe(0);
		expect(reads.openCount).toBe(0);
	});

	test("asks the relay with exactly the app's headers, so a range comes back as 206 with its content range", async () => {
		const { reads, sent } = relayed();
		open(reads, 's2', '/media/video.mp4', { authorization: 'Nostr get', range: 'bytes=10-19' });
		const body = await readEverythingGrantingCredit(reads, sent, 's2');
		expect(heard.at(-1)).toEqual({ method: 'GET', path: '/media/video.mp4', host, authorization: 'Nostr get', range: 'bytes=10-19' });
		expect(sent[0]).toMatchObject({ status: 206, headers: { 'content-range': `bytes 10-19/${video.byteLength}` } });
		expect(Buffer.compare(body, video.slice(10, 20))).toBe(0);
	});

	test('passes a refusal from the relay through as its answer', async () => {
		const { reads, sent } = relayed();
		open(reads, 's3', '/media/video.mp4', { authorization: 'Nostr stolen' });
		const body = await readEverythingGrantingCredit(reads, sent, 's3');
		expect(sent[0]).toMatchObject({ kind: 'media.head', status: 401 });
		expect(new TextDecoder().decode(body)).toBe('auth required');
	});

	test('answers a HEAD with its status and headers and no body', async () => {
		const { reads, sent } = relayed();
		open(reads, 's4', '/media/video.mp4', { authorization: 'Nostr get' }, 'HEAD');
		await until(() => sent.some((one) => one.kind === 'media.end'), 'the HEAD to end');
		expect(sent.map((one) => one.kind)).toEqual(['media.head', 'media.end']);
		expect(heard.at(-1)?.method).toBe('HEAD');
	});

	test('sends no more than the credit the gateway granted, and goes on when it grants more', async () => {
		const { reads, sent } = relayed();
		open(reads, 's5', '/media/endless-credit', { authorization: 'Nostr get' });
		await until(() => bodyBytesOf(sent) === mediaCreditWindowBytes, 'the first window to be sent');
		await Bun.sleep(50);
		expect(bodyBytesOf(sent)).toBe(mediaCreditWindowBytes);

		reads.take({ kind: 'media.credit', streamID: 's5', bytes: 1000 });
		await until(() => bodyBytesOf(sent) === mediaCreditWindowBytes + 1000, 'the granted bytes to be sent');
		await Bun.sleep(50);
		expect(bodyBytesOf(sent)).toBe(mediaCreditWindowBytes + 1000);
		reads.take({ kind: 'media.stop', streamID: 's5', reason: 'done' });
	});

	test('aborts the relay read when the app stops reading, saying nothing more and logging nothing', async () => {
		const { reads, sent, reported } = relayed();
		open(reads, 's6', '/media/endless-stop', { authorization: 'Nostr get' });
		await until(() => bodyBytesOf(sent) === mediaCreditWindowBytes, 'the window to be sent');
		const before = sent.length;
		reads.take({ kind: 'media.stop', streamID: 's6', reason: 'the app stopped reading' });
		await until(() => endlessAborted.includes('/media/endless-stop'), 'the relay read to be aborted');
		await until(() => reads.openCount === 0, 'the read to be let go');
		expect(sent.length).toBe(before);
		expect(reported).toEqual([]);
	});

	test('aborts every relay read when the gateway connection goes, logging each one with its path', async () => {
		const { reads, sent, reported } = relayed();
		open(reads, 's7', '/media/endless-gone', { authorization: 'Nostr get' });
		await until(() => bodyBytesOf(sent) > 0, 'the body to start');
		reads.closeEverything();
		await until(() => endlessAborted.includes('/media/endless-gone'), 'the relay read to be aborted');
		await until(() => reported.length > 0, 'the failure to be logged');
		expect(reported).toEqual(['messenger media GET /media/endless-gone failed: the gateway connection went away']);
	});

	test('tells the gateway and logs one line when the relay cannot be reached', async () => {
		const unreachable = Bun.serve({ hostname: '127.0.0.1', port: 0, fetch: () => new Response() });
		const port = unreachable.port;
		unreachable.stop(true);
		const { reads, sent, reported } = mediaReadsOver(`http://127.0.0.1:${port}`);
		open(reads, 's8', '/media/video.mp4', { authorization: 'Nostr get' });
		await until(() => sent.length > 0, 'the failure to reach the gateway');
		expect(sent).toEqual([expect.objectContaining({ kind: 'media.fail', streamID: 's8' })]);
		expect(reported).toHaveLength(1);
		expect(reported[0]).toStartWith('messenger media GET /media/video.mp4 failed: ');
	});
});
