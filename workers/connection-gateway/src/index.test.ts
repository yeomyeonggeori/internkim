import { afterAll, afterEach, beforeAll, describe, expect, jest, test } from 'bun:test';
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
const issuer = 'https://issuer.test/auth/v1';
const hostCompanyID = 'host-company';
let signingKey: CryptoKey;
let keyServer: ReturnType<typeof Bun.serve>;

function base64URL(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function encodeSegment(document: unknown): string {
	return base64URL(new TextEncoder().encode(JSON.stringify(document)));
}

async function signedHostToken(claims: Record<string, unknown>): Promise<string> {
	const signed = `${encodeSegment({ alg: 'ES256', kid: 'host-test-key' })}.${encodeSegment(claims)}`;
	const signature = await crypto.subtle.sign(
		{ name: 'ECDSA', hash: 'SHA-256' },
		signingKey,
		new TextEncoder().encode(signed)
	);
	return `${signed}.${base64URL(new Uint8Array(signature))}`;
}

function newState(values: Map<string, unknown> = new Map()): DurableObjectState {
	const storage = {
		get: (key: string) => Promise.resolve(values.get(key)),
		put: (key: string, value: unknown) => {
			values.set(key, value);
			return Promise.resolve();
		},
		delete: (key: string) => Promise.resolve(values.delete(key))
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

function hostEnvironment(object: CompanyConnectionObject): WorkerEnvironment {
	return {
		...environmentReaching(object),
		SUPABASE_URL: 'https://issuer.test',
		SUPABASE_PUBLISHABLE_KEY: 'publishable',
		SUPABASE_JWKS_URL: `${keyServer.url}jwks`
	};
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

beforeAll(async () => {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
	signingKey = pair.privateKey;
	const publicKey = await crypto.subtle.exportKey('jwk', pair.publicKey);
	const publicKeyDocument = JSON.stringify({ keys: [{ ...publicKey, kid: 'host-test-key' }] });
	keyServer = Bun.serve({
		port: 0,
		fetch(request) {
			const isJWKS = new URL(request.url).pathname === '/jwks';
			return new Response(isJWKS ? publicKeyDocument : 'not found', { status: isJWKS ? 200 : 404 });
		}
	});
});

afterAll(() => keyServer.stop());

afterEach(() => {
	jest.useRealTimers();
});

describe('the public fetch handler', () => {
	test('requires a bearer token for the host route', async () => {
		const object = new CompanyConnectionObject(newState());
		const response = await worker.fetch(
			new Request(`https://gateway/company/${hostCompanyID}/host`, { headers: { Upgrade: 'websocket' } }),
			hostEnvironment(object)
		);
		expect(response.status).toBe(401);
	});

	test('refuses malformed, expired, wrong issuer, spoofed, and wrong-company tokens', async () => {
		const cases = [
			{ token: 'not-a-jwt', status: 401 },
			{
				token: await signedHostToken({ sub: 'account-1', iss: issuer, exp: 1, app_metadata: { company_id: hostCompanyID } }),
				status: 401
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: 'https://other.test/auth/v1', exp: 4102444800, app_metadata: { company_id: hostCompanyID } }),
				status: 401
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: issuer, exp: 4102444800, user_metadata: { company_id: hostCompanyID } }),
				status: 403
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: issuer, exp: 4102444800, app_metadata: { company_id: 'another-company' } }),
				status: 403
			}
		];
		for (const entry of cases) {
			const object = new CompanyConnectionObject(newState());
			const response = await worker.fetch(
				new Request(`https://gateway/company/${hostCompanyID}/host`, {
					headers: { Upgrade: 'websocket', Authorization: `Bearer ${entry.token}` }
				}),
				hostEnvironment(object)
			);
			expect(response.status).toBe(entry.status);
		}
	});

	test('accepts a signed host token and keeps the internal path private', async () => {
		const object = new CompanyConnectionObject(newState());
		const token = await signedHostToken({
			sub: 'account-1',
			iss: issuer,
			exp: 4102444800,
			app_metadata: { company_id: hostCompanyID }
		});
		const accepted = await worker.fetch(
			new Request(`https://gateway/company/${hostCompanyID}/host`, {
				headers: { Upgrade: 'websocket', Authorization: `Bearer ${token}` }
			}),
			hostEnvironment(object)
		);
		expect(accepted.status).toBe(101);
		const privatePath = await worker.fetch(
			new Request(`https://gateway/company/${hostCompanyID}/host-session`, {
				headers: { Upgrade: 'websocket', Authorization: 'Bearer anything' }
			}),
			hostEnvironment(object)
		);
		expect(privatePath.status).toBe(404);
	});

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

describe('the server key at rest', () => {
	function serverRequest(key: string): Request {
		return new Request(`https://gateway/company/${companyID}/server`, {
			headers: { Upgrade: 'websocket', Authorization: `Bearer ${key}` }
		});
	}

	async function sha256Hex(value: string): Promise<string> {
		const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
		return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
	}

	test('keeps only a digest of the key', async () => {
		const values = new Map<string, unknown>();
		const object = new CompanyConnectionObject(newState(values));
		await object.fetch(
			new Request(`https://gateway/company/${companyID}/server-key`, {
				method: 'POST',
				body: JSON.stringify({ serverKey })
			})
		);
		expect([...values.values()]).not.toContain(serverKey);
		expect([...values.values()]).toContain(await sha256Hex(serverKey));
		expect((await object.fetch(serverRequest(serverKey))).status).not.toBe(401);
		expect((await object.fetch(serverRequest('another-key'))).status).toBe(401);
	});

	test('accepts a key stored in plaintext once and replaces it with its digest', async () => {
		const values = new Map<string, unknown>([['serverKey', serverKey]]);
		const object = new CompanyConnectionObject(newState(values));
		expect((await object.fetch(serverRequest('another-key'))).status).toBe(401);
		expect(values.get('serverKey')).toBe(serverKey);

		expect((await object.fetch(serverRequest(serverKey))).status).not.toBe(401);
		expect(values.has('serverKey')).toBe(false);
		expect(values.get('serverKeyDigest')).toBe(await sha256Hex(serverKey));
		expect((await object.fetch(serverRequest(serverKey))).status).not.toBe(401);
		expect((await object.fetch(serverRequest('another-key'))).status).toBe(401);
	});

	test('refuses a wrong key of the same length', async () => {
		const { object } = await companyWithAServerKey();
		const sameLength = 'x'.repeat(serverKey.length);
		expect(sameLength).toHaveLength(serverKey.length);
		expect((await object.fetch(serverRequest(sameLength))).status).toBe(401);
	});

	test('refuses a header that is not a bearer credential', async () => {
		const values = new Map<string, unknown>([['serverKey', serverKey]]);
		const object = new CompanyConnectionObject(newState(values));
		const response = await object.fetch(
			new Request(`https://gateway/company/${companyID}/server`, {
				headers: { Upgrade: 'websocket', Authorization: serverKey }
			})
		);
		expect(response.status).toBe(401);
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
