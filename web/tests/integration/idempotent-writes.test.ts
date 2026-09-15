import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { controlPlane, issuePersonalAccessToken, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `idempotent-writes-${Date.now()}`;

let companyID = '';
let memberID = '';
let writersToken = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Idempotent Writes Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	memberID = provisioned.adminMemberID;

	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-admin@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	writersToken = await issuePersonalAccessToken(client, memberID, 'writer', 'delete');
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

type RouteAnswer = { status: number; body: unknown };

async function invoke(name: string, body: Record<string, unknown>): Promise<RouteAnswer> {
	const path = `/tools/${name}/invoke`;
	const request = new Request(`https://space.example.test/api/v1${path}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${writersToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	try {
		const response = await reachTheAPI({
			request,
			url: new URL(request.url),
			params: { path: path.replace(/^\//, '') },
			platform: undefined
		} as unknown as Parameters<typeof reachTheAPI>[0]);
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, body: refusal.body };
	}
}

async function tasksTitled(title: string): Promise<number> {
	const { count } = await client
		.from('task')
		.select('id', { count: 'exact', head: true })
		.eq('company_id', companyID)
		.eq('title', title);
	return count ?? 0;
}

async function keysRemembered(): Promise<{ key: string; tool_name: string }[]> {
	const { data } = await client.from('idempotency_key').select('key, tool_name').eq('member_id', memberID);
	return data ?? [];
}

describe('a write that carries an idempotency key', () => {
	const title = `${slug} 분기 보고서 초안`;
	const key = `${slug}-first-key`;

	test('is written once and answered the same way twice', async () => {
		const first = await invoke('task_add', { input: { title }, idempotencyKey: key });
		const again = await invoke('task_add', { input: { title }, idempotencyKey: key });

		expect(first.status).toBe(200);
		expect(again).toEqual(first);
		expect(await tasksTitled(title)).toBe(1);
		expect(await keysRemembered()).toEqual([{ key, tool_name: 'task_add' }]);
	});

	test('is written again under another key', async () => {
		const otherTitle = `${slug} 분기 보고서 검토`;
		const other = await invoke('task_add', { input: { title: otherTitle }, idempotencyKey: `${slug}-second-key` });

		expect(other.status).toBe(200);
		expect(await tasksTitled(otherTitle)).toBe(1);
		expect((await keysRemembered()).map((remembered) => remembered.key)).toContain(`${slug}-second-key`);
	});

	test('refuses the key of another tool rather than answering with its record', async () => {
		const answered = await invoke('task_update', {
			input: { taskHint: title, title: `${title} (수정)` },
			idempotencyKey: key
		});

		expect(answered.status).toBe(409);
	});

	test('is not remembered for a read', async () => {
		const listed = await invoke('task_list', { input: {}, idempotencyKey: `${slug}-read-key` });

		expect(listed.status).toBe(200);
		expect((await keysRemembered()).map((remembered) => remembered.key)).not.toContain(`${slug}-read-key`);
	});

	test('is not remembered when the record refuses the write', async () => {
		const refused = await invoke('task_update', {
			input: { taskHint: `${slug} names no task`, title: '이름 없는 업무' },
			idempotencyKey: `${slug}-refused-key`
		});

		expect(refused.status).toBeGreaterThanOrEqual(400);
		expect((await keysRemembered()).map((remembered) => remembered.key)).not.toContain(`${slug}-refused-key`);
	});

	test('records clock out once and returns its saved answer on a repeated key', async () => {
		const clockIn = await invoke('attendance_add', {
			input: { kind: 'clock_in' }, idempotencyKey: `${slug}-clock-in`
		});
		expect(clockIn.status).toBe(200);
		const clockOutRequest = { input: { kind: 'clock_out' }, idempotencyKey: `${slug}-clock-out` };
		const clockOut = await invoke('attendance_add', clockOutRequest);
		const repeatedClockOut = await invoke('attendance_add', clockOutRequest);
		expect(clockOut.status).toBe(200);
		expect(repeatedClockOut).toEqual(clockOut);
		const attendance = await client.from('attendance').select('kind').eq('member_id', memberID);
		if (attendance.error) throw new Error(attendance.error.message);
		expect(attendance.data).toHaveLength(2);
		expect(attendance.data).toEqual(expect.arrayContaining([{ kind: 'clock_in' }, { kind: 'clock_out' }]));
	});
});
