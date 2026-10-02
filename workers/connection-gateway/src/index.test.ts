import { afterAll, afterEach, beforeAll, describe, expect, jest, test } from 'bun:test';
import worker, { CompanyCalls, CompanyConnectionObject, type WorkerEnvironment } from './index';
import { ConnectionObject, TestSocket, newState, socketHeldByTheObject } from './test-durable-object';
import { oneShotCallBoundMilliseconds, waitingCallsPerCompany, type OneShotAnswer } from './routing';
import { isPageNavigation } from './messenger';

const serverKey = 'company-server-key';
const companyID = 'c1';
const callBody = { method: 'POST', path: '/tools/message_send/invoke', requester: 'someone@example.com' };
const hostCompanyID = 'host-company';
let recordURL = '';
let issuer = '';
let companyTheRecordGrants: string | null = hostCompanyID;
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
		SUPABASE_URL: recordURL,
		SUPABASE_PUBLISHABLE_KEY: 'publishable',
		SUPABASE_JWKS_URL: `${keyServer.url}jwks`
	};
}

async function companyWithAServerKey(): Promise<{ calls: CompanyCalls; object: CompanyConnectionObject }> {
	const object = new ConnectionObject(newState());
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

const storedBlob = new TextEncoder().encode('the bytes of a picture');
const uploadsKept: { contentType: string | null; body: string }[] = [];

function storedBlobAnswer(request: Request): Response {
	const range = request.headers.get('Range');
	if (range !== 'bytes=4-8') return new Response(storedBlob, { headers: { 'content-length': String(storedBlob.byteLength) } });
	return new Response(storedBlob.slice(4, 9), {
		status: 206,
		headers: { 'content-range': `bytes 4-8/${storedBlob.byteLength}`, 'content-length': '5' }
	});
}

async function keepUpload(request: Request): Promise<Response> {
	uploadsKept.push({ contentType: request.headers.get('Content-Type'), body: await request.text() });
	return Response.json({ Key: 'staged' });
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
			const path = new URL(request.url).pathname;
			if (path === '/jwks') return new Response(publicKeyDocument);
			if (path === '/rest/v1/rpc/my_app_company') return Response.json(companyTheRecordGrants);
			if (path === '/rest/v1/rpc/company_of_messenger_address') {
				const slug = new URL(request.url).searchParams.get('address_slug');
				return Response.json(slug === 'acme' || slug === 'acme-media' || slug === 'acme-upload' ? companyID : null);
			}
			if (path === '/store/blob') return storedBlobAnswer(request);
			if (path === '/store/upload') return keepUpload(request);
			return new Response('not found', { status: 404 });
		}
	});
	recordURL = keyServer.url.href.replace(/\/+$/, '');
	issuer = `${recordURL}/auth/v1`;
});

afterAll(() => keyServer.stop());

afterEach(() => {
	jest.useRealTimers();
});

describe('the public fetch handler', () => {
	test('requires a bearer token for the host route', async () => {
		const object = new ConnectionObject(newState());
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
				token: await signedHostToken({ sub: 'account-1', iss: issuer, aud: 'authenticated', exp: 1, app_metadata: { company_id: hostCompanyID } }),
				status: 401
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: 'https://other.test/auth/v1', aud: 'authenticated', exp: 4102444800, app_metadata: { company_id: hostCompanyID } }),
				status: 401
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: issuer, aud: 'authenticated', exp: 4102444800, user_metadata: { company_id: hostCompanyID } }),
				status: 403
			},
			{
				token: await signedHostToken({ sub: 'account-1', iss: issuer, aud: 'authenticated', exp: 4102444800, app_metadata: { company_id: 'another-company' } }),
				status: 403
			}
		];
		for (const entry of cases) {
			const object = new ConnectionObject(newState());
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
		const object = new ConnectionObject(newState());
		const token = await signedHostToken({
			sub: 'account-1',
			iss: issuer, aud: 'authenticated',
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

	test('refuses a signed host token once the company has moved off that computer', async () => {
		const token = await signedHostToken({
			sub: 'account-1',
			iss: issuer, aud: 'authenticated',
			exp: 4102444800,
			app_metadata: { company_id: hostCompanyID }
		});
		companyTheRecordGrants = null;
		try {
			const refused = await worker.fetch(
				new Request(`https://gateway/company/${hostCompanyID}/host`, {
					headers: { Upgrade: 'websocket', Authorization: `Bearer ${token}` }
				}),
				hostEnvironment(new ConnectionObject(newState()))
			);
			expect(refused.status).toBe(403);
		} finally {
			companyTheRecordGrants = hostCompanyID;
		}
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

		const sameLength = await worker.fetch(
			new Request(address, { method: 'POST', headers: { Authorization: 'Bearer the-tokex' } }),
			environment
		);
		expect(sameLength.status).toBe(401);

		const emptyToken = await worker.fetch(
			new Request(address, { method: 'POST', headers: { Authorization: 'Bearer ' } }),
			{ GATEWAY_ADMIN_TOKEN: '' } as WorkerEnvironment
		);
		expect(emptyToken.status).toBe(401);

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
		const object = new ConnectionObject(newState(values));
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
		const object = new ConnectionObject(newState(values));
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
		const object = new ConnectionObject(newState(values));
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

describe('a company messenger address', () => {
	async function messengerCompany(): Promise<{ object: CompanyConnectionObject; host: TestSocket }> {
		const object = new ConnectionObject(newState());
		await object.fetch(
			new Request(`https://connection-gateway/company/${companyID}/host-session`, { headers: { Upgrade: 'websocket' } })
		);
		if (!socketHeldByTheObject) throw new Error('the object accepted no host socket');
		return { object, host: socketHeldByTheObject };
	}

	async function callTheHostReceived(host: TestSocket): Promise<{ requestID: string; capability: string; body: Record<string, unknown> }> {
		for (let round = 0; round < 50; round += 1) {
			const call = host.documents().findLast((document) => (document as { kind?: string }).kind === 'call');
			if (call) return call as { requestID: string; capability: string; body: Record<string, unknown> };
			await Bun.sleep(2);
		}
		throw new Error('the host was asked for nothing');
	}

	function relayAnswer(requestID: string, status: number, body: Record<string, unknown>): string {
		return JSON.stringify({ kind: 'result', requestID, status, body });
	}

	test('opens a stream to the company that holds the address', async () => {
		const { object, host } = await messengerCompany();
		const response = await worker.fetch(
			new Request('https://acme.example.test/', { headers: { Upgrade: 'websocket', 'CF-Connecting-IP': '198.51.100.7' } }),
			hostEnvironment(object)
		);
		expect(response.status).toBe(101);
		expect(host.documents()).toEqual([
			expect.objectContaining({ kind: 'stream.open', host: 'acme.example.test', path: '/', forwardedFor: '198.51.100.7' })
		]);
	});

	test('answers 404 at an address no company holds, and asks no computer', async () => {
		const { object, host } = await messengerCompany();
		const response = await worker.fetch(
			new Request('https://stranger.example.test/', { headers: { Upgrade: 'websocket' } }),
			hostEnvironment(object)
		);
		expect(response.status).toBe(404);
		expect(host.sent).toEqual([]);
	});

	test('carries an http request to the relay and its answer back, leaving the edge headers behind', async () => {
		const { object, host } = await messengerCompany();
		const answered = worker.fetch(
			new Request('https://acme.example.test/query?limit=2', {
				method: 'POST',
				headers: { Authorization: 'Nostr abc', 'Content-Type': 'application/json', 'CF-Ray': 'ray', 'X-Forwarded-For': '1.2.3.4' },
				body: '{"kinds":[1]}'
			}),
			hostEnvironment(object)
		);
		const call = await callTheHostReceived(host);
		expect(call.capability).toBe('messenger.http');
		expect(call.body).toEqual({
			method: 'POST',
			path: '/query?limit=2',
			host: 'acme.example.test',
			headers: { authorization: 'Nostr abc', 'content-type': 'application/json' },
			bodyBase64: btoa('{"kinds":[1]}')
		});
		host.receive(
			relayAnswer(call.requestID, 200, {
				headers: { 'content-type': 'application/json', 'content-encoding': 'gzip' },
				bodyBase64: btoa('[]')
			})
		);
		const response = await answered;
		expect(response.status).toBe(200);
		expect(response.headers.get('content-encoding')).toBeNull();
		expect(await response.text()).toBe('[]');
	});

	test('streams a blob from the transfer store, honouring the range the app asked for', async () => {
		const { object, host } = await messengerCompany();
		const sha = 'a'.repeat(64);
		const answered = worker.fetch(
			new Request(`https://acme-media.example.test/media/${sha}.png`, {
				headers: { Authorization: 'Nostr get-token', Range: 'bytes=4-8' }
			}),
			hostEnvironment(object)
		);
		const call = await callTheHostReceived(host);
		expect(call.capability).toBe('messenger.media.read');
		expect(call.body).toMatchObject({ method: 'GET', path: `/media/${sha}.png`, headers: { authorization: 'Nostr get-token', range: 'bytes=4-8' } });
		host.receive(
			relayAnswer(call.requestID, 200, {
				headers: { 'content-type': 'image/png', 'cache-control': 'private, max-age=31536000, immutable' },
				bodyBase64: '',
				objectURL: `${recordURL}/store/blob`
			})
		);
		const response = await answered;
		expect(response.status).toBe(206);
		expect(response.headers.get('content-range')).toBe(`bytes 4-8/${storedBlob.byteLength}`);
		expect(response.headers.get('content-type')).toBe('image/png');
		expect(await response.text()).toBe('bytes');
	});

	test('passes a refusal from the relay through without touching the store', async () => {
		const { object, host } = await messengerCompany();
		const answered = worker.fetch(
			new Request(`https://acme-media.example.test/media/${'b'.repeat(64)}`),
			hostEnvironment(object)
		);
		const call = await callTheHostReceived(host);
		host.receive(relayAnswer(call.requestID, 401, { headers: { 'content-type': 'text/plain' }, bodyBase64: btoa('auth required') }));
		const response = await answered;
		expect(response.status).toBe(401);
		expect(await response.text()).toBe('auth required');
	});

	test('stages an upload in the transfer store, then has the relay take it from there', async () => {
		const { object, host } = await messengerCompany();
		const answered = worker.fetch(
			new Request('https://acme-upload.example.test/upload', {
				method: 'PUT',
				headers: { Authorization: 'Nostr upload-token', 'Content-Type': 'image/png', 'X-SHA-256': 'c'.repeat(64) },
				body: 'picture bytes'
			}),
			hostEnvironment(object)
		);
		const stage = await callTheHostReceived(host);
		expect(stage.capability).toBe('messenger.media.stage');
		host.receive(relayAnswer(stage.requestID, 200, { headers: {}, bodyBase64: '', uploadURL: `${recordURL}/store/upload`, stagedPath: 'c1/shared/staged' }));

		let write = stage;
		for (let round = 0; round < 50 && write.requestID === stage.requestID; round += 1) {
			await Bun.sleep(2);
			write = await callTheHostReceived(host);
		}
		expect(write.capability).toBe('messenger.media.write');
		expect(write.body).toMatchObject({ method: 'PUT', path: '/upload', stagedPath: 'c1/shared/staged', headers: { 'x-sha-256': 'c'.repeat(64) } });
		expect(uploadsKept.at(-1)).toEqual({ contentType: 'image/png', body: 'picture bytes' });

		host.receive(relayAnswer(write.requestID, 200, { headers: { 'content-type': 'application/json' }, bodyBase64: btoa('{"sha256":"c"}') }));
		const response = await answered;
		expect(response.status).toBe(200);
		expect(await response.json()).toEqual({ sha256: 'c' });
	});

	test('sends a person who opens the address in a browser to the zone, as every other company hostname does', async () => {
		const { object, host } = await messengerCompany();
		const response = await worker.fetch(
			new Request('https://acme.example.test/settings?tab=people', { headers: { Accept: 'text/html,application/xhtml+xml' } }),
			hostEnvironment(object)
		);
		expect(response.status).toBe(308);
		expect(response.headers.get('Location')).toBe('https://example.test/settings?tab=people');
		expect(host.sent).toEqual([]);
	});

	test('keeps the relay information document and media reads, which a browser may also ask for, at the relay', () => {
		const navigation = { Accept: 'text/html,image/avif,*/*' };
		expect(isPageNavigation(new Request('https://acme.example.test/', { headers: { Accept: 'application/nostr+json' } }), '/')).toBe(false);
		expect(isPageNavigation(new Request(`https://acme.example.test/media/${'a'.repeat(64)}.png`, { headers: navigation }), `/media/${'a'.repeat(64)}.png`)).toBe(false);
		expect(isPageNavigation(new Request('https://acme.example.test/', { headers: { ...navigation, Upgrade: 'websocket' } }), '/')).toBe(false);
		expect(isPageNavigation(new Request('https://acme.example.test/', { method: 'POST', headers: navigation }), '/')).toBe(false);
	});

	test('answers 503 for an http request while the company computer is away', async () => {
		const object = new ConnectionObject(newState());
		const response = await worker.fetch(new Request('https://acme.example.test/info'), hostEnvironment(object));
		expect(response.status).toBe(503);
	});
});
