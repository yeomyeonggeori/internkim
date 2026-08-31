import { beforeEach, describe, expect, test } from 'bun:test';
import worker, { type CompanyAnswer, type CompanyCall, type WorkerEnvironment } from './index';
import { largestFileAnAttachmentCanBe } from './files';

type Row = { name: string; permission: string; member: { id: string; email: string; company_id: string } };

let recordAnswers: Row[] = [];

let storeStatuses: number[] = [];
let storePuts: string[] = [];

type RecordCall = { url: string; method: string; body: unknown };

let recordCalls: RecordCall[] = [];
let credentialRows: unknown[] = [];

Object.assign(globalThis, {
	fetch: async (url: string, options: { method?: string; body?: string } = {}) => {
		if (String(url).includes('/storage/v1/object/')) {
			storePuts.push(String(url));
			const status = storeStatuses.shift() ?? 200;
			return { ok: status < 400, status, json: async () => null };
		}
		const method = options.method ?? 'GET';
		recordCalls.push({ url: String(url), method, body: options.body ? JSON.parse(options.body) : null });
		if (method !== 'GET') return { ok: true, status: 200, json: async () => credentialRows };
		if (String(url).includes('external_id=')) {
			return { ok: true, status: 200, json: async () => recordAnswers };
		}
		return { ok: true, status: 200, json: async () => credentialRows };
	}
});

function keyHeldBy(
	permission: string,
	email = 'someone@example.com',
	companyID = 'c1',
	name = 'laptop'
): void {
	recordAnswers = [{ name, permission, member: { id: 'm1', email, company_id: companyID } }];
}

type CompanyCallRecord = { companyID: string; call: CompanyCall };

let carried: CompanyCallRecord[] = [];
let companyAnswer: CompanyAnswer = { requestID: '', status: 200, body: { ok: true } };

const environment: WorkerEnvironment = {
	COMPANY_CALLS: {
		async callCompany(companyID, call) {
			carried.push({ companyID, call });
			return { ...companyAnswer, requestID: call.requestID };
		}
	},
	SUPABASE_URL: 'https://plane.supabase.co',
	SUPABASE_SECRET_KEY: 'service-role'
};

function call(path: string, key: string | null, options: RequestInit = {}): Promise<Response> {
	const headers = key ? { Authorization: `Bearer ${key}`, ...options.headers } : options.headers;
	return worker.fetch(new Request(`https://api.intern.kim${path}`, { ...options, headers }), environment);
}

beforeEach(() => {
	carried = [];
	storeStatuses = [];
	storePuts = [];
	recordCalls = [];
	credentialRows = [];
	companyAnswer = { requestID: '', status: 200, body: { ok: true } };
});

describe('a call that names nobody', () => {
	test('is refused when it carries no token at all', async () => {
		const response = await call('/v1/tools', null);
		expect(response.status).toBe(401);
		expect(await response.json()).toEqual({ error: 'this call carried no token' });
		expect(carried).toHaveLength(0);
	});

	test('is refused when the header carries an empty bearer', async () => {
		const response = await call('/v1/tools', '   ');
		expect(response.status).toBe(401);
	});

	test('is refused when the record holds no such token', async () => {
		recordAnswers = [];
		const response = await call('/v1/tools', 'ik_never-issued');
		expect(response.status).toBe(401);
		expect(await response.json()).toEqual({ error: 'this token belongs to nobody' });
		expect(carried).toHaveLength(0);
	});

	test('is refused when the bearer is not a personal access token', async () => {
		const response = await call('/v1/tools', 'eyJhbGciOiJIUzI1NiJ9.nope');
		expect(response.status).toBe(401);
	});
});

describe('discovery, answered at the edge', () => {
	async function toolNamesFor(permission: string, key: string): Promise<string[]> {
		keyHeldBy(permission);
		const response = await call('/v1/tools', key);
		expect(response.status).toBe(200);
		expect(response.headers.get('X-INTERNKIM-CATALOG')).toBe('base');
		const answer = (await response.json()) as { tools: { name: string }[] };
		return answer.tools.map((descriptor) => descriptor.name);
	}

	test('shows a read key only what it can read, and reaches no company', async () => {
		const names = await toolNamesFor('read', 'ik_reader');
		expect(names).toContain('task_list');
		expect(names).not.toContain('task_add');
		expect(names).not.toContain('task_delete');
		expect(carried).toHaveLength(0);
	});

	test('shows a write key the writes as well', async () => {
		const names = await toolNamesFor('write', 'ik_writer');
		expect(names).toContain('task_add');
		expect(names).toContain('message_send');
		expect(names).not.toContain('message_delete');
	});

	test('shows a delete key the deletions too', async () => {
		const names = await toolNamesFor('delete', 'ik_deleter');
		expect(names).toContain('message_delete');
		expect(names).toContain('site_unserve');
	});

	test('says plainly that this is the base catalog and where the live set lives', async () => {
		keyHeldBy('delete');
		const response = await call('/v1/tools', 'ik_asks-what-this-is');
		const answer = (await response.json()) as { catalog: { source: string; live: string; explanation: string } };
		expect(answer.catalog.source).toBe('base');
		expect(answer.catalog.live).toBe('/v1/tools?live=true');
		expect(answer.catalog.explanation).toContain('companion');
	});

	test('answers one descriptor the key reaches', async () => {
		keyHeldBy('read');
		const response = await call('/v1/tools/task_list', 'ik_reads-one');
		expect(response.status).toBe(200);
		expect(response.headers.get('X-INTERNKIM-CATALOG')).toBe('base');
		expect((await response.json()) as { name: string }).toMatchObject({ name: 'task_list' });
		expect(carried).toHaveLength(0);
	});

	test('answers 404 for a tool above the key, as if it did not exist', async () => {
		keyHeldBy('write');
		const response = await call('/v1/tools/task_delete', 'ik_cannot-delete');
		expect(response.status).toBe(404);
		expect(carried).toHaveLength(0);
	});

	test('goes to the company when the caller asks for the live set', async () => {
		keyHeldBy('delete');
		companyAnswer = { requestID: '', status: 200, body: { tools: [] } };
		const response = await call('/v1/tools?live=true&namespace=browser', 'ik_wants-live');
		expect(response.status).toBe(200);
		expect(carried).toHaveLength(1);
		expect(carried[0].call.body).toMatchObject({ method: 'GET', path: '/tools', query: 'namespace=browser' });
	});
});

describe('everything else, carried to the company', () => {
	test('sends the call the contract describes, naming the requester the key resolved to', async () => {
		keyHeldBy('write', 'Writer@Example.com', 'company-7');
		const response = await call('/v1/tools/message_send/invoke?dry=1', 'ik_invokes', {
			method: 'POST',
			body: JSON.stringify({ input: { text: 'hello' } })
		});

		expect(response.status).toBe(200);
		expect(carried).toHaveLength(1);
		expect(carried[0].companyID).toBe('company-7');
		expect(carried[0].call.capability).toBe('person.api.request');
		expect(carried[0].call.requestID).toMatch(/^[0-9a-f-]{36}$/);
		expect(carried[0].call.body).toEqual({
			method: 'POST',
			path: '/tools/message_send/invoke',
			query: 'dry=1',
			permission: 'write',
			requester: 'writer@example.com',
			payload: { input: { text: 'hello' } }
		});
	});

	test('names a fresh request each time, so two calls never collide', async () => {
		keyHeldBy('write');
		await call('/v1/agent/messages', 'ik_asks-twice', { method: 'POST', body: '{}' });
		await call('/v1/agent/messages', 'ik_asks-twice', { method: 'POST', body: '{}' });
		expect(carried[0].call.requestID).not.toBe(carried[1].call.requestID);
	});

	test('answers with the status and body the company gave', async () => {
		keyHeldBy('delete');
		companyAnswer = { requestID: '', status: 503, body: { error: 'server_offline' } };
		const response = await call('/v1/tools/task_delete/invoke', 'ik_reaches-nobody', { method: 'POST', body: '{}' });
		expect(response.status).toBe(503);
		expect(await response.json()).toEqual({ error: 'server_offline' });
	});

	test('answers 502 when what came back is not a status', async () => {
		keyHeldBy('delete');
		companyAnswer = { requestID: '', status: 9000, body: null };
		const response = await call('/v1/tools/task_add/invoke', 'ik_gets-nonsense', { method: 'POST', body: '{}' });
		expect(response.status).toBe(502);
		expect((await response.json()) as { error: string }).toMatchObject({ error: expect.stringContaining('9000') });
	});

	test('refuses a body that is not a json object rather than passing it on', async () => {
		keyHeldBy('write');
		const response = await call('/v1/tools/task_add/invoke', 'ik_sends-garbage', { method: 'POST', body: 'not json' });
		expect(response.status).toBe(400);
		expect(carried).toHaveLength(0);
	});
});

describe('a path this worker does not serve', () => {
	test('is not found, so the route it is bound to is the only thing it answers', async () => {
		keyHeldBy('delete');
		const response = await call('/api/agent/host-session', 'ik_wrong-path');
		expect(response.status).toBe(404);
		expect(carried).toHaveLength(0);
	});
});

describe('putting a file where a message can attach it', () => {
	const bytes = 'a file worth sending';

	function put(key: string, options: RequestInit & { query?: string } = {}): Promise<Response> {
		const { query, ...request } = options;
		return call(`/v1/files${query ?? ''}`, key, {
			method: 'POST',
			headers: { 'Content-Type': 'image/png' },
			body: bytes,
			...request
		});
	}

	test('is refused for a key that may only read, and keeps nothing', async () => {
		keyHeldBy('read');

		const response = await put('ik_reader');

		expect(response.status).toBe(403);
		expect(storePuts).toHaveLength(0);
		expect(carried).toHaveLength(0);
	});

	test('is refused at the length the caller claims, naming the ceiling', async () => {
		keyHeldBy('write');

		const response = await put('ik_writer', {
			headers: { 'Content-Type': 'image/png', 'Content-Length': String(largestFileAnAttachmentCanBe + 1) }
		});

		expect(response.status).toBe(413);
		expect((await response.json() as { error: string }).error).toContain(
			String(largestFileAnAttachmentCanBe)
		);
		expect(storePuts).toHaveLength(0);
		expect(carried).toHaveLength(0);
	});

	test('is refused when the body itself is over the ceiling, whatever the header said', async () => {
		keyHeldBy('write');

		const response = await put('ik_writer', {
			body: new Uint8Array(largestFileAnAttachmentCanBe + 1)
		});

		expect(response.status).toBe(413);
		expect(storePuts).toHaveLength(0);
		expect(carried).toHaveLength(0);
	});

	test('is refused when the call carried no file at all', async () => {
		keyHeldBy('write');

		const response = await put('ik_writer', { body: '' });

		expect(response.status).toBe(400);
		expect(storePuts).toHaveLength(0);
	});

	test('keeps the bytes, then asks the company to materialise them, carrying no bytes', async () => {
		keyHeldBy('write');
		companyAnswer = {
			requestID: '',
			status: 200,
			body: { file: { path: '/workspace/private/people/person-1/inbox/api/mascot.png', sizeBytes: 20 } }
		};

		const response = await put('ik_writer', { query: '?filename=mascot.png' });

		expect(response.status).toBe(200);
		expect(await response.json()).toEqual(companyAnswer.body);
		expect(storePuts).toHaveLength(1);
		expect(storePuts[0]).toStartWith(
			'https://plane.supabase.co/storage/v1/object/asset/c1/shared/attachment/'
		);
		expect(storePuts[0]).toEndWith('.png');

		expect(carried).toHaveLength(1);
		expect(carried[0]?.companyID).toBe('c1');
		expect(carried[0]?.call.capability).toBe('person.api.file');
		const body = carried[0]?.call.body as Record<string, unknown>;
		expect(body.requester).toBe('someone@example.com');
		expect(body.permission).toBe('write');
		expect(body.contentType).toBe('image/png');
		expect(body.filename).toBe('mascot.png');
		expect(String(body.digest)).toMatch(/^[0-9a-f]{64}$/);
		expect(JSON.stringify(carried[0])).not.toContain(bytes);
	});

	test('lands on one address when the same bytes are sent twice', async () => {
		keyHeldBy('write');
		storeStatuses = [200, 409];

		await put('ik_writer');
		await put('ik_writer');

		expect(storePuts).toHaveLength(2);
		expect(storePuts[0]).toBe(storePuts[1]);
		expect(carried).toHaveLength(2);
		expect((carried[0]?.call.body as { digest: string }).digest).toBe(
			(carried[1]?.call.body as { digest: string }).digest
		);
	});

	test('answers 502 when the asset store refuses, and asks the company nothing', async () => {
		keyHeldBy('write');
		storeStatuses = [503];

		const response = await put('ik_writer');

		expect(response.status).toBe(502);
		expect(carried).toHaveLength(0);
	});
});

describe('a token makes and revokes tokens', () => {
	function mint(token: string, body: unknown): Promise<Response> {
		return call('/v1/tokens', token, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
	}

	test('mints one for the member the presented token belongs to, and never asks the company', async () => {
		keyHeldBy('write', 'someone@example.com', 'c1', 'laptop');

		const response = await mint('ik_writer', { name: 'ci' });

		expect(response.status).toBe(200);
		const answered = (await response.json()) as { name: string; permission: string; token: string };
		expect(answered.name).toBe('ci');
		expect(answered.permission).toBe('write');
		expect(answered.token).toStartWith('ik_');
		expect(carried).toHaveLength(0);
		const written = recordCalls.find((made) => made.method === 'POST');
		expect((written?.body as { member_id: string }).member_id).toBe('m1');
		expect(JSON.stringify(written?.body)).not.toContain(answered.token);
	});

	test('takes the presented token rung when none is asked for', async () => {
		keyHeldBy('read');
		const answered = (await (await mint('ik_reader', { name: 'ci' })).json()) as { permission: string };
		expect(answered.permission).toBe('read');
	});

	test('refuses to mint a rung above the one presented', async () => {
		keyHeldBy('read');
		const response = await mint('ik_reader', { name: 'ci', permission: 'delete' });
		expect(response.status).toBe(403);
		expect(recordCalls.some((made) => made.method === 'POST')).toBe(false);
	});

	test('mints a rung below the one presented', async () => {
		keyHeldBy('delete');
		const response = await mint('ik_admin', { name: 'ci', permission: 'read' });
		expect(response.status).toBe(200);
		expect(((await response.json()) as { permission: string }).permission).toBe('read');
	});

	test('refuses a nameless token and one named after the token making the call', async () => {
		keyHeldBy('write', 'someone@example.com', 'c1', 'laptop');
		expect((await mint('ik_writer', {})).status).toBe(400);
		expect((await mint('ik_writer', { name: 'laptop' })).status).toBe(409);
	});

	test('refuses a rung that is not on the ladder', async () => {
		keyHeldBy('delete');
		const response = await mint('ik_admin', { name: 'ci', permission: 'root' });
		expect(response.status).toBe(400);
	});

	test('lists the tokens the member holds', async () => {
		keyHeldBy('read');
		credentialRows = [{ name: 'ci', permission: 'read' }];

		const response = await call('/v1/tokens', 'ik_reader');

		expect(response.status).toBe(200);
		expect(await response.json()).toEqual({ tokens: [{ name: 'ci', permission: 'read' }] });
		expect(carried).toHaveLength(0);
	});

	test('revokes another token of the same member', async () => {
		keyHeldBy('read', 'someone@example.com', 'c1', 'laptop');
		credentialRows = [{ name: 'ci' }];

		const response = await call('/v1/tokens?name=ci', 'ik_reader', { method: 'DELETE' });

		expect(response.status).toBe(200);
		expect(await response.json()).toEqual({ forgotten: 'ci' });
		expect(recordCalls.some((made) => made.method === 'DELETE')).toBe(true);
	});

	test('will not revoke the token making the call', async () => {
		keyHeldBy('delete', 'someone@example.com', 'c1', 'laptop');

		const response = await call('/v1/tokens?name=laptop', 'ik_admin', { method: 'DELETE' });

		expect(response.status).toBe(409);
		expect(recordCalls.some((made) => made.method === 'DELETE')).toBe(false);
	});

	test('answers 404 when no token of the member goes by that name', async () => {
		keyHeldBy('delete', 'someone@example.com', 'c1', 'laptop');
		credentialRows = [];

		const response = await call('/v1/tokens?name=gone', 'ik_admin', { method: 'DELETE' });

		expect(response.status).toBe(404);
	});

	test('is refused without a token of its own', async () => {
		const response = await call('/v1/tokens', null);
		expect(response.status).toBe(401);
	});
});
