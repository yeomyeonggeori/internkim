import { afterEach, describe, expect, jest, test } from 'bun:test';
import { hostLacksHeader } from './company-connection';
import {
	binaryMediaBody,
	hostSupportsHeader,
	largestStreamFrameBytes,
	mediaBody,
	mediaCreditWindowBytes,
	messengerMediaStreamCapability
} from './host-protocol';
import { mediaReadsPerCompany, mediaStallBoundMilliseconds, relayContentLengthHeader } from './media-reads';
import { ConnectionObject, TestSocket, TestState, socketHeldByTheObject } from './test-durable-object';

const publicHost = 'acme-media.example.test';
const mediaPath = `/media/${'a'.repeat(64)}.mp4`;
const appHeaders = { authorization: 'Nostr get-token', range: 'bytes=0-' };

type Opened = {
	kind: string;
	streamID: string;
	method: string;
	path: string;
	host: string;
	headers: Record<string, string>;
	creditBytes: number;
	bodyFrames?: string;
};

afterEach(() => {
	jest.useRealTimers();
});

async function connectHost(object: ConnectionObject, supports: string | null = messengerMediaStreamCapability): Promise<TestSocket> {
	const headers: Record<string, string> = { Upgrade: 'websocket' };
	if (supports) headers[hostSupportsHeader] = supports;
	await object.fetch(new Request('https://connection-gateway/company/c1/host-session', { headers }));
	if (!socketHeldByTheObject) throw new Error('the object accepted no host socket');
	return socketHeldByTheObject;
}

function askForMedia(object: ConnectionObject, method = 'GET'): Promise<Response> {
	return object.fetch(
		new Request('https://connection-gateway/company/c1/messenger-media', {
			method: 'POST',
			body: JSON.stringify({ method, path: mediaPath, host: publicHost, headers: appHeaders })
		})
	);
}

async function flushMicrotasks(): Promise<void> {
	for (let round = 0; round < 20; round += 1) await Promise.resolve();
}

async function openedOn(host: TestSocket, nth = 0): Promise<Opened> {
	for (let round = 0; round < 50; round += 1) {
		const opened = host.documents().filter((document) => (document as { kind?: string }).kind === 'media.open')[nth];
		if (opened) return opened as Opened;
		await flushMicrotasks();
	}
	throw new Error('the host was asked for no media');
}

function hostSends(host: TestSocket, message: Record<string, unknown>): void {
	host.receive(JSON.stringify(message));
}

function creditGranted(host: TestSocket, streamID: string): number {
	return host
		.documents()
		.filter((document): document is { kind: string; streamID: string; bytes: number } => (document as { kind?: string }).kind === 'media.credit')
		.filter((credit) => credit.streamID === streamID)
		.reduce((total, credit) => total + credit.bytes, 0);
}

function stopsSentFor(host: TestSocket, streamID: string): unknown[] {
	return host.documents().filter((document) => {
		const record = document as { kind?: string; streamID?: string };
		return record.kind === 'media.stop' && record.streamID === streamID;
	});
}

function patterned(length: number, seed: number): Uint8Array<ArrayBuffer> {
	return Uint8Array.from({ length }, (_, index) => (index * 31 + seed) % 251);
}

describe('a media read', () => {
	test('asks the host for the media with the app headers unchanged, the credit window, and binary body frames', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		void askForMedia(object);
		expect(await openedOn(host)).toEqual({
			kind: 'media.open',
			streamID: expect.any(String),
			method: 'GET',
			path: mediaPath,
			host: publicHost,
			headers: appHeaders,
			creditBytes: mediaCreditWindowBytes,
			bodyFrames: 'binary'
		});
	});

	test('answers as soon as the host sends the status, and carries a body several frames long intact and in order', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object);
		const { streamID } = await openedOn(host);
		const frames = [0, 1, 2, 3, 4].map((seed) => patterned(largestStreamFrameBytes, seed));
		const total = frames.reduce((sum, frame) => sum + frame.byteLength, 0);
		hostSends(host, {
			kind: 'media.head',
			streamID,
			status: 206,
			headers: { 'content-type': 'video/mp4', 'content-range': `bytes 0-${total - 1}/${total}`, 'content-length': String(total) }
		});
		const response = await answered;
		expect(response.status).toBe(206);
		expect(response.headers.get('content-range')).toBe(`bytes 0-${total - 1}/${total}`);

		const reader = response.body?.getReader();
		if (!reader) throw new Error('the answer has no body');
		const received: Uint8Array[] = [];
		for (const frame of frames) {
			const read = reader.read();
			host.receive(mediaBody(streamID, frame));
			const { value } = await read;
			if (value) received.push(value);
		}
		const finished = reader.read();
		hostSends(host, { kind: 'media.end', streamID });
		expect((await finished).done).toBe(true);
		expect(Buffer.compare(Buffer.concat(received), Buffer.concat(frames))).toBe(0);
	});

	test('carries a body several binary frames long intact and in order', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object);
		const { streamID } = await openedOn(host);
		const frames = [0, 1, 2, 3, 4].map((seed) => patterned(largestStreamFrameBytes, seed));
		const total = frames.reduce((sum, frame) => sum + frame.byteLength, 0);
		hostSends(host, {
			kind: 'media.head',
			streamID,
			status: 206,
			headers: { 'content-type': 'video/mp4', 'content-range': `bytes 0-${total - 1}/${total}`, 'content-length': String(total) }
		});
		const response = await answered;
		expect(response.status).toBe(206);
		expect(response.headers.get('content-range')).toBe(`bytes 0-${total - 1}/${total}`);

		const reader = response.body?.getReader();
		if (!reader) throw new Error('the answer has no body');
		const received: Uint8Array[] = [];
		for (const frame of frames) {
			const read = reader.read();
			host.receive(binaryMediaBody(streamID, frame).buffer);
			const { value } = await read;
			if (value) received.push(value);
		}
		const finished = reader.read();
		hostSends(host, { kind: 'media.end', streamID });
		expect((await finished).done).toBe(true);
		expect(Buffer.compare(Buffer.concat(received), Buffer.concat(frames))).toBe(0);
	});

	test('grants the host credit only for bytes the app has read, so at most one window waits in the object', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object);
		const { streamID } = await openedOn(host);
		hostSends(host, { kind: 'media.head', streamID, status: 200, headers: {} });
		const response = await answered;
		const quarter = mediaCreditWindowBytes / 4;
		for (let seed = 0; seed < 4; seed += 1) host.receive(mediaBody(streamID, patterned(quarter, seed)));
		await flushMicrotasks();
		expect(creditGranted(host, streamID)).toBe(0);

		const reader = response.body?.getReader();
		if (!reader) throw new Error('the answer has no body');
		await reader.read();
		expect(creditGranted(host, streamID)).toBe(quarter);
		await reader.read();
		await reader.read();
		expect(creditGranted(host, streamID)).toBe(3 * quarter);
		await reader.cancel();
	});

	test('keeps streaming past 60 seconds as long as bytes keep coming, and gives up on 60 seconds of silence', async () => {
		jest.useFakeTimers();
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object);
		const { streamID } = await openedOn(host);
		hostSends(host, { kind: 'media.head', streamID, status: 200, headers: {} });
		const reader = (await answered).body?.getReader();
		if (!reader) throw new Error('the answer has no body');

		const received: Uint8Array[] = [];
		for (let seed = 0; seed < 5; seed += 1) {
			const read = reader.read();
			await flushMicrotasks();
			jest.advanceTimersByTime(mediaStallBoundMilliseconds - 1_000);
			host.receive(mediaBody(streamID, patterned(1024, seed)));
			const { value } = await read;
			if (value) received.push(value);
		}
		expect(received).toHaveLength(5);

		const stalled = reader.read();
		await flushMicrotasks();
		jest.advanceTimersByTime(mediaStallBoundMilliseconds + 1);
		await expect(stalled).rejects.toThrow('sent nothing for 60 seconds');
		await flushMicrotasks();
		expect(stopsSentFor(host, streamID)).toHaveLength(1);
	});

	test('tells the host to stop when the app stops reading', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object);
		const { streamID } = await openedOn(host);
		hostSends(host, { kind: 'media.head', streamID, status: 200, headers: { 'content-length': '10000000' } });
		const response = await answered;
		await response.body?.cancel();
		await flushMicrotasks();
		expect(stopsSentFor(host, streamID)).toEqual([{ kind: 'media.stop', streamID, reason: 'the app stopped reading' }]);
	});

	test('answers a HEAD with the relay status and length and no body', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const answered = askForMedia(object, 'HEAD');
		const { streamID } = await openedOn(host);
		hostSends(host, { kind: 'media.head', streamID, status: 200, headers: { 'content-type': 'video/mp4', 'content-length': '1234' } });
		hostSends(host, { kind: 'media.end', streamID });
		const response = await answered;
		expect(response.status).toBe(200);
		expect(response.headers.get(relayContentLengthHeader)).toBe('1234');
		expect(response.body).toBeNull();
		expect(stopsSentFor(host, streamID)).toEqual([]);
	});

	test('fails the read the host says failed, and every read when the host goes away', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const refused = askForMedia(object);
		const first = await openedOn(host);
		hostSends(host, { kind: 'media.fail', streamID: first.streamID, reason: 'the relay is down' });
		expect((await refused).status).toBe(502);
		expect(await (await refused).json()).toEqual({ error: 'the relay is down' });

		const cut = askForMedia(object);
		const second = await openedOn(host, 1);
		hostSends(host, { kind: 'media.head', streamID: second.streamID, status: 200, headers: {} });
		const reading = (await cut).body?.getReader().read();
		await host.drop();
		await expect(reading).rejects.toThrow('the company computer went away');
	});

	test('is sent the one-shot way to a host that does not announce streaming', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object, null);
		const response = await askForMedia(object);
		expect(response.headers.get(hostLacksHeader)).toBe('messenger.media.stream');
		expect(host.documents()).toEqual([]);
	});

	test(`holds at most ${mediaReadsPerCompany} reads open for a company at once`, async () => {
		const object = new ConnectionObject(new TestState());
		await connectHost(object);
		const open = Array.from({ length: mediaReadsPerCompany }, () => askForMedia(object));
		await flushMicrotasks();
		const refused = await askForMedia(object);
		expect(refused.status).toBe(503);
		expect(open).toHaveLength(mediaReadsPerCompany);
	});
});
