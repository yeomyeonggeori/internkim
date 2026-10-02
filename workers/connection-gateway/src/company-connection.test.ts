import { describe, expect, test } from 'bun:test';
import { streamAddressHeader, streamHostHeader, streamPathHeader } from './company-connection';
import { largestStreamFrameBytes, pingFrame, pongFrame } from './host-protocol';
import { ConnectionObject, TestSocket, TestState, socketHeldByTheObject } from './test-durable-object';

const companyID = 'c1';
const publicHost = 'acme.example.test';

function heldSocket(): TestSocket {
	if (!socketHeldByTheObject) throw new Error('the object accepted no socket');
	return socketHeldByTheObject;
}

async function connectHost(object: ConnectionObject): Promise<TestSocket> {
	const accepted = await object.fetch(
		new Request(`https://connection-gateway/company/${companyID}/host-session`, { headers: { Upgrade: 'websocket' } })
	);
	expect(accepted.status).toBe(101);
	return heldSocket();
}

async function connectMember(object: ConnectionObject, memberID: string): Promise<TestSocket> {
	await object.fetch(
		new Request(`https://connection-gateway/company/${companyID}/client`, {
			headers: { Upgrade: 'websocket', 'x-internkim-member': memberID }
		})
	);
	return heldSocket();
}

async function openApp(object: ConnectionObject, path = '/'): Promise<{ app: TestSocket; response: Response }> {
	const response = await object.fetch(
		new Request(`https://connection-gateway/company/${companyID}/stream-session`, {
			headers: {
				Upgrade: 'websocket',
				[streamHostHeader]: publicHost,
				[streamPathHeader]: path,
				[streamAddressHeader]: '192.0.2.10'
			}
		})
	);
	return { app: heldSocket(), response };
}

function streamIDOpenedOn(host: TestSocket): string {
	const opened = host.documents().filter(isRecordOfKind('stream.open'));
	const last = opened.at(-1);
	if (!last || typeof last.streamID !== 'string') throw new Error('the host was told of no stream');
	return last.streamID;
}

function isRecordOfKind(kind: string): (document: unknown) => document is Record<string, unknown> {
	return (document): document is Record<string, unknown> =>
		typeof document === 'object' && document !== null && 'kind' in document && document.kind === kind;
}

describe('an app stream', () => {
	test('is refused while the company computer is away', async () => {
		const object = new ConnectionObject(new TestState());
		const { response } = await openApp(object);
		expect(response.status).toBe(503);
	});

	test('opens on the host socket naming the address the app dialled', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { response } = await openApp(object, '/huddle/channel-1/audio');
		expect(response.status).toBe(101);
		expect(host.documents().filter(isRecordOfKind('stream.open'))).toEqual([
			{
				kind: 'stream.open',
				streamID: streamIDOpenedOn(host),
				host: publicHost,
				path: '/huddle/channel-1/audio',
				forwardedFor: '192.0.2.10'
			}
		]);
	});

	test('carries frames both ways untouched, keyed by its stream', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		const streamID = streamIDOpenedOn(host);

		app.receive('["REQ","s1",{"kinds":[1]}]');
		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toEqual([
			{ kind: 'stream.frame', streamID, data: '["REQ","s1",{"kinds":[1]}]' }
		]);

		host.receive(JSON.stringify({ kind: 'stream.frame', streamID, data: '["EOSE","s1"]' }));
		expect(app.sent).toEqual(['["EOSE","s1"]']);
	});

	test('carries a binary frame as bytes', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		const streamID = streamIDOpenedOn(host);

		app.receive(new Uint8Array([1, 2, 255]).buffer);
		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toEqual([
			{ kind: 'stream.frame', streamID, data: 'AQL/', isBinary: true }
		]);

		host.receive(JSON.stringify({ kind: 'stream.frame', streamID, data: 'AQL/', isBinary: true }));
		expect(app.sentBinary.map((bytes) => [...bytes])).toEqual([[1, 2, 255]]);
	});

	test('keeps two apps apart', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const first = await openApp(object);
		const firstID = streamIDOpenedOn(host);
		const second = await openApp(object);
		const secondID = streamIDOpenedOn(host);
		expect(firstID).not.toBe(secondID);

		host.receive(JSON.stringify({ kind: 'stream.frame', streamID: secondID, data: 'for the second' }));
		expect(first.app.sent).toEqual([]);
		expect(second.app.sent).toEqual(['for the second']);
	});

	test('closes with the code the relay closed with', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		host.receive(JSON.stringify({ kind: 'stream.close', streamID: streamIDOpenedOn(host), code: 4001, reason: 'restricted' }));
		expect(app.closedWith).toEqual({ code: 4001, reason: 'restricted' });
	});

	test('tells the host when the app goes', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		const streamID = streamIDOpenedOn(host);
		await app.drop(1001, 'going away');
		expect(host.documents().filter(isRecordOfKind('stream.close'))).toEqual([
			{ kind: 'stream.close', streamID, code: 1001, reason: 'going away' }
		]);
	});

	test('answers a frame for an app that has gone with a close, once', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		host.receive(JSON.stringify({ kind: 'stream.frame', streamID: 'gone', data: 'late' }));
		host.receive(JSON.stringify({ kind: 'stream.close', streamID: 'gone', code: 1000, reason: '' }));
		expect(host.documents().filter(isRecordOfKind('stream.close'))).toEqual([
			{ kind: 'stream.close', streamID: 'gone', code: 1000, reason: 'the app has gone' }
		]);
	});
});

describe('a frame over the relay limit', () => {
	test('never enters the envelope and closes the stream as too big', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		const streamID = streamIDOpenedOn(host);

		app.receive('x'.repeat(largestStreamFrameBytes + 1));

		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toEqual([]);
		expect(app.closedWith?.code).toBe(1009);
		expect(host.documents().filter(isRecordOfKind('stream.close'))).toEqual([
			{ kind: 'stream.close', streamID, code: 1009, reason: 'a frame may carry at most 512 KiB' }
		]);
	});

	test('is measured in bytes, so a frame of multibyte text that fits in characters is still refused', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		app.receive('가'.repeat(Math.ceil(largestStreamFrameBytes / 3) + 1));
		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toEqual([]);
		expect(app.closedWith?.code).toBe(1009);
	});

	test('a frame exactly at the limit is carried', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const { app } = await openApp(object);
		app.receive('x'.repeat(largestStreamFrameBytes));
		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toHaveLength(1);
		expect(app.closedWith).toBeNull();
	});
});

describe('when the company computer drops', () => {
	test('every app stream closes as a restart, so the apps reconnect', async () => {
		const object = new ConnectionObject(new TestState());
		const host = await connectHost(object);
		const first = await openApp(object);
		const second = await openApp(object);
		const member = await connectMember(object, 'm1');

		await host.drop();

		expect(first.app.closedWith?.code).toBe(1012);
		expect(second.app.closedWith?.code).toBe(1012);
		expect(member.closedWith).toBeNull();
		expect(member.documents().at(-1)).toMatchObject({ kind: 'presence', isServerConnected: false });
	});

	test('a second computer taking over closes the first one\'s streams', async () => {
		const object = new ConnectionObject(new TestState());
		const first = await connectHost(object);
		const { app } = await openApp(object);
		const second = await connectHost(object);

		expect(first.closedWith?.code).toBe(1012);
		expect(app.closedWith?.code).toBe(1012);
		first.receive(JSON.stringify({ kind: 'result', requestID: 'late', status: 200, body: null }));
		await first.drop();
		const member = await connectMember(object, 'm1');
		expect(member.documents()).toEqual([expect.objectContaining({ kind: 'presence', isServerConnected: true })]);
		expect(second.closedWith).toBeNull();
	});
});

describe('hibernation', () => {
	test('answers the ping the host keeps the connection alive with, without waking the object', () => {
		const state = new TestState();
		new ConnectionObject(state);
		expect(state.autoResponse).toEqual(expect.objectContaining({ request: pingFrame, response: pongFrame }));
	});

	test('a member\'s call outlives the object sleeping, and its answer still reaches them', async () => {
		const state = new TestState();
		const object = new ConnectionObject(state);
		const host = await connectHost(object);
		const member = await connectMember(object, 'm1');
		member.receive(JSON.stringify({ kind: 'call', requestID: 'r1', capability: 'person.runs.list', body: {} }));
		expect(host.documents().filter(isRecordOfKind('call'))).toHaveLength(1);

		const woken = state.wake();
		new ConnectionObject(woken);
		member.receive(JSON.stringify({ kind: 'call', requestID: 'r1', capability: 'person.runs.list', body: {} }));
		expect(host.documents().filter(isRecordOfKind('call'))).toHaveLength(1);

		host.receive(JSON.stringify({ kind: 'result', requestID: 'r1', status: 200, body: { runs: [] } }));
		expect(member.documents().filter(isRecordOfKind('result'))).toEqual([
			{ kind: 'result', requestID: 'r1', status: 200, body: { runs: [] } }
		]);
	});

	test('an app stream keeps flowing after the object sleeps', async () => {
		const state = new TestState();
		const object = new ConnectionObject(state);
		const host = await connectHost(object);
		const { app } = await openApp(object);
		const streamID = streamIDOpenedOn(host);

		new ConnectionObject(state.wake());
		app.receive('["EVENT",{}]');
		host.receive(JSON.stringify({ kind: 'stream.frame', streamID, data: '["OK"]' }));

		expect(host.documents().filter(isRecordOfKind('stream.frame'))).toEqual([
			{ kind: 'stream.frame', streamID, data: '["EVENT",{}]' }
		]);
		expect(app.sent).toEqual(['["OK"]']);
	});

	test('when the computer was last heard is still known after it left and the object slept', async () => {
		const state = new TestState();
		const object = new ConnectionObject(state);
		const host = await connectHost(object);
		state.pongedAt.set(host, new Date(4_102_444_800_000));
		await host.drop();

		const woken = state.wake();
		const member = await connectMember(new ConnectionObject(woken), 'm1');
		expect(member.documents()).toEqual([{ kind: 'presence', isServerConnected: false, serverSeenAt: 4_102_444_800_000 }]);
	});
});
