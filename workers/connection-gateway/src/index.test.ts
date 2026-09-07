import { afterEach, describe, expect, jest, test } from 'bun:test';
import worker, { CompanyCalls, CompanyConnectionObject, type WorkerEnvironment } from './index';
import { oneShotCallBoundMilliseconds, waitingCallsPerCompany, type OneShotAnswer } from './routing';

class TestSocket {
	readonly sent: string[] = [];
	private readonly listeners = new Map<string, ((event: MessageEvent) => void)[]>();

	accept(): void {}

	close(): void {}

	send(document: string): void {
		this.sent.push(document);
	}

	addEventListener(name: string, handler: (event: MessageEvent) => void): void {
		this.listeners.set(name, [...(this.listeners.get(name) ?? []), handler]);
	}

	receive(document: string): void {
		for (const handler of this.listeners.get('message') ?? []) {
			handler({ data: document } as MessageEvent);
		}
	}
}

let socketHeldByTheObject: TestSocket | null = null;

class TestWebSocketPair {
	readonly 0 = new TestSocket();
	readonly 1 = new TestSocket();

	constructor() {
		socketHeldByTheObject = this[1];
	}
}

Object.assign(globalThis, { WebSocketPair: TestWebSocketPair });

const serverKey = 'company-server-key';
const companyID = 'c1';
const callBody = { method: 'POST', path: '/tools/message_send/invoke', requester: 'someone@example.com' };

function newState(): DurableObjectState {
	const values = new Map<string, unknown>();
	const storage = {
		get: (key: string) => Promise.resolve(values.get(key)),
		put: (key: string, value: unknown) => {
			values.set(key, value);
			return Promise.resolve();
		}
	};
	return { storage } as unknown as DurableObjectState;
}

function environmentReaching(object: CompanyConnectionObject): WorkerEnvironment {
	const namespace = {
		idFromName: (name: string) => name,
		get: () => ({ fetch: (request: Request) => object.fetch(request) })
	};
	return { COMPANY_CONNECTIONS: namespace } as unknown as WorkerEnvironment;
}

async function companyWithAServerKey(): Promise<{ calls: CompanyCalls; object: CompanyConnectionObject }> {
	const object = new CompanyConnectionObject(newState());
	await object.fetch(
		new Request(`https://gateway/company/${companyID}/server-key`, {
			method: 'POST',
			body: JSON.stringify({ serverKey })
		})
	);
	return { calls: new CompanyCalls({} as ExecutionContext, environmentReaching(object)), object };
}

async function connectedCompany(): Promise<{ calls: CompanyCalls; serverSocket: TestSocket }> {
	const { calls, object } = await companyWithAServerKey();
	await object.fetch(
		new Request(`https://gateway/company/${companyID}/server`, {
			headers: { Upgrade: 'websocket', Authorization: `Bearer ${serverKey}` }
		})
	);
	if (!socketHeldByTheObject) throw new Error('the object accepted no server socket');
	return { calls, serverSocket: socketHeldByTheObject };
}

async function flushMicrotasks(): Promise<void> {
	for (let round = 0; round < 20; round += 1) await Promise.resolve();
}

function answerFor(requestID: string, status: number, body: unknown): string {
	return JSON.stringify({ kind: 'result', requestID, status, body });
}

afterEach(() => {
	jest.useRealTimers();
});

describe('the public fetch handler', () => {
	test('refuses a one-shot call that does not hold the gateway token', async () => {
		const environment = { GATEWAY_ADMIN_TOKEN: 'the-token' } as WorkerEnvironment;
		const address = `https://gateway/company/${companyID}/call`;

		const bare = await worker.fetch(new Request(address, { method: 'POST' }), environment);
		expect(bare.status).toBe(401);

		const wrong = await worker.fetch(
			new Request(address, { method: 'POST', headers: { Authorization: 'Bearer another' } }),
			environment
		);
		expect(wrong.status).toBe(401);

		const unconfigured = await worker.fetch(
			new Request(address, { method: 'POST', headers: { Authorization: 'Bearer the-token' } }),
			{} as WorkerEnvironment
		);
		expect(unconfigured.status).toBe(401);
	});

	test('a trailing slash meets the same refusal', async () => {
		const response = await worker.fetch(
			new Request(`https://gateway/company/${companyID}/call/`, { method: 'POST' }),
			{} as WorkerEnvironment
		);
		expect(response.status).toBe(401);
	});
});

describe('CompanyCalls.callCompany', () => {
	test('answers server_offline when no company server is connected', async () => {
		const { calls } = await companyWithAServerKey();
		const call = { requestID: 'r1', capability: 'person.api.request', body: callBody };
		expect(await calls.callCompany(companyID, call)).toEqual({
			requestID: 'r1',
			status: 503,
			body: { error: 'server_offline' }
		});
	});

	test('carries the call to the server socket and returns the answer it matches', async () => {
		const { calls, serverSocket } = await connectedCompany();
		const answered = calls.callCompany(companyID, {
			requestID: 'r1',
			capability: 'person.api.request',
			body: callBody
		});
		await flushMicrotasks();

		expect(serverSocket.sent.map((document) => JSON.parse(document))).toEqual([
			{ kind: 'call', requestID: 'r1', capability: 'person.api.request', body: callBody }
		]);

		serverSocket.receive(answerFor('r1', 200, { eventID: 'e1' }));
		expect(await answered).toEqual({ requestID: 'r1', status: 200, body: { eventID: 'e1' } });
	});

	test('answers 504 when the call outlives its bound', async () => {
		jest.useFakeTimers();
		const { calls } = await connectedCompany();
		const answered = calls.callCompany(companyID, { requestID: 'r1', capability: 'person.api.request' });
		await flushMicrotasks();

		jest.advanceTimersByTime(oneShotCallBoundMilliseconds + 1);
		expect(await answered).toEqual({ requestID: 'r1', status: 504, body: { error: 'call_timed_out' } });
	});

	test('refuses a call beyond the calls one company may keep waiting', async () => {
		const { calls, serverSocket } = await connectedCompany();
		const waiting: Promise<OneShotAnswer>[] = [];
		for (let index = 0; index < waitingCallsPerCompany; index += 1) {
			waiting.push(calls.callCompany(companyID, { requestID: `r${index}`, capability: 'person.api.request' }));
		}
		await flushMicrotasks();

		expect(await calls.callCompany(companyID, { requestID: 'one-too-many', capability: 'person.api.request' })).toEqual({
			requestID: 'one-too-many',
			status: 429,
			body: { error: 'too_many_waiting_calls' }
		});
		expect(serverSocket.sent).toHaveLength(waitingCallsPerCompany);

		for (let index = 0; index < waitingCallsPerCompany; index += 1) {
			serverSocket.receive(answerFor(`r${index}`, 200, null));
		}
		expect((await Promise.all(waiting)).map((answer) => answer.status)).toEqual(
			Array.from({ length: waitingCallsPerCompany }, () => 200)
		);
	});

	test('refuses a call naming no capability', async () => {
		const { calls } = await companyWithAServerKey();
		await expect(calls.callCompany(companyID, { requestID: 'r1', capability: '  ' })).rejects.toThrow(TypeError);
	});
});

describe('what the server delivers unasked', () => {
	test('reaches the audience as the event alone, without the audience list', async () => {
		const { object } = await companyWithAServerKey();
		await object.fetch(
			new Request(`https://gateway/company/${companyID}/server`, {
				headers: { Upgrade: 'websocket', Authorization: `Bearer ${serverKey}` }
			})
		);
		const serverSocket = socketHeldByTheObject;
		if (!serverSocket) throw new Error('the object accepted no server socket');
		await object.fetch(
			new Request(`https://gateway/company/${companyID}/client`, {
				headers: { Upgrade: 'websocket', 'x-internkim-member': 'm1' }
			})
		);
		const clientSocket = socketHeldByTheObject;
		if (!clientSocket || clientSocket === serverSocket) throw new Error('the object accepted no client socket');

		serverSocket.receive(
			JSON.stringify({
				kind: 'deliver',
				event: { kind: 'message.arrived', conversationID: 'channel-1' },
				audienceMemberIDs: ['m1', 'm2']
			})
		);

		const delivered = clientSocket.sent.map((document) => JSON.parse(document)).filter((frame) => frame.kind === 'deliver');
		expect(delivered).toEqual([{ kind: 'deliver', event: { kind: 'message.arrived', conversationID: 'channel-1' } }]);
	});
});
