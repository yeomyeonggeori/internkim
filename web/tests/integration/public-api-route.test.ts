import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { z } from 'zod';
import {
	addMember,
	asMember,
	controlPlane,
	issuePersonalAccessToken,
	provisionCompany,
	sessionForMember,
} from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { createOpenApiDocument } from '../../../docs/web/app/lib/openapi';
import { savedAttendanceEventSchema } from '../../src/lib/attendance/recorded-attendance';
import { taskBoardResultSchema } from '../../src/lib/server/public-api/catalog/tools';
import { taskWeekOfDate } from '../../src/lib/task/task-week-code';
import { createMockFetch } from '../unit/test-fetch';
import { moveAttendanceEarlier } from '../support/move-attendance-earlier';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { GET: listTokens } = await import('../../src/routes/api/v1/tokens/+server');
const { POST: mintToken, DELETE: revokeToken } = await import('../../src/routes/api/v1/token/+server');
const { fallback: reachTheAPI } = await import('../../src/routes/api/v1/[...path]/+server');
const { POST: reachMCP } = await import('../../src/routes/api/v1/mcp/+server');
const { POST: resetPassword } = await import('../../src/routes/api/member/password-reset/+server');
const { POST: issueCalendarFeed } = await import('../../src/routes/api/calendar/subscription/+server');
const { POST: removeMember } = await import('../../src/routes/api/member/remove/+server');
const { POST: invitePerson } = await import('../../src/routes/api/member/invite/+server');
const { GET: readDataRoom } = await import('../../src/routes/api/v1/data-room/[companyID]/+server');
const { GET: readDataRoomLink, POST: unlockDataRoomLink } = await import('../../src/routes/api/v1/data-room/links/[linkID]/+server');
const { POST: acceptDataRoom } = await import('../../src/routes/api/v1/data-room/invitations/[shareID]/+server');
const { POST: sendDataRoomInvitation } = await import('../../src/routes/api/v1/data-room/invitations/[shareID]/send/+server');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `public-api-${Date.now()}`;

let companyID = '';
let memberID = '';
let holdersToken = '';
let readersToken = '';
let departedToken = '';
let sessionToken = '';
let administratorsToken = '';
let administratorSession = '';
let dataRoomInvitationID = '';
let dataRoomLinkID = '';
let dataRoomLinkCookie = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Public API Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	const { data: administrator } = await client.auth.admin.createUser({
		email: `${slug}-admin@example.test`,
		email_confirm: true
	});
	await client
		.from('member')
		.update({ user_id: administrator.user!.id, status: 'active' })
		.eq('id', provisioned.adminMemberID);
	administratorsToken = await issuePersonalAccessToken(client, provisioned.adminMemberID, 'administrator', 'delete');

	memberID = await addMember(client, companyID, `${slug}-holder@example.test`);

	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-holder@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);

	holdersToken = await issuePersonalAccessToken(client, memberID, 'holder', 'delete');
	readersToken = await issuePersonalAccessToken(client, memberID, 'reader', 'read');

	const departedID = await addMember(client, companyID, `${slug}-departed@example.test`);
	departedToken = await issuePersonalAccessToken(client, departedID, 'departed', 'delete');
	await client.from('member').update({ status: 'departed' }).eq('id', departedID);

	sessionToken = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID)).accessToken;
	administratorSession = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, provisioned.adminMemberID)).accessToken;
	const administratorCaller = asMember({ projectURL, publishableKey }, administratorSession);
	const invitation = await administratorCaller.rpc('data_room_share_create', {
		target_company: companyID, circle_id: 'investor', audience: 'email', recipient_email: `${slug}-holder@example.test`
	});
	if (invitation.error) throw new Error(invitation.error.message);
	dataRoomInvitationID = z.string().uuid().parse(invitation.data);
	const link = await administratorCaller.rpc('data_room_link_create', {
		target_company: companyID, circle_id: 'investor', label: 'Sample investor review', access_code: '123456'
	});
	if (link.error) throw new Error(link.error.message);
	dataRoomLinkID = z.string().uuid().parse(link.data);
	const request = asking(`/data-room/links/${dataRoomLinkID}`, null, {
		method: 'POST', headers: { Origin: 'https://space.example.test', 'Content-Type': 'application/json' },
		body: JSON.stringify({ accessCode: '123456', noticeVersion: '1' })
	});
	const opened = await unlockDataRoomLink({ request, url: new URL(request.url), params: { linkID: dataRoomLinkID },
		platform: undefined, cookies: { get: () => undefined, set: (_name, value) => { dataRoomLinkCookie = value; } },
		getClientAddress: () => '127.0.0.1' });
	expect(opened.status).toBe(200);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function asking(path: string, token: string | null, options: RequestInit = {}): Request {
	const headers = token ? { Authorization: `Bearer ${token}`, ...options.headers } : options.headers;
	return new Request(`https://space.example.test/api/v1${path}`, { ...options, headers });
}

type RouteAnswer = { status: number; body: unknown };

async function answerOf(handler: () => Promise<Response>): Promise<RouteAnswer> {
	try {
		const response = await handler();
		return { status: response.status, body: await response.json() };
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: unknown };
		if (typeof refusal.status !== 'number') throw thrown;
		return { status: refusal.status, body: refusal.body };
	}
}

function reach(
	path: string,
	token: string | null,
	options: RequestInit = {},
	backgroundWork?: (work: Promise<unknown>) => void
): Promise<RouteAnswer> {
	const request = asking(path, token, options);
	const url = new URL(request.url);
	return answerOf(() =>
		Promise.resolve(
			reachTheAPI({
				request,
				url,
				params: { path: path.replace(/^\//, '').split('?')[0] },
				platform: backgroundWork ? { context: { waitUntil: backgroundWork } } : undefined
			} as unknown as Parameters<typeof reachTheAPI>[0])
		)
	);
}

function tokens(token: string): Promise<RouteAnswer> {
	const request = asking('/tokens', token);
	return answerOf(() =>
		Promise.resolve(listTokens({ request, platform: undefined } as unknown as Parameters<typeof listTokens>[0]))
	);
}

function mint(token: string, body: unknown): Promise<RouteAnswer> {
	const request = asking('/token', token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	return answerOf(() =>
		Promise.resolve(mintToken({ request, platform: undefined } as unknown as Parameters<typeof mintToken>[0]))
	);
}

function revoke(token: string, name: string): Promise<RouteAnswer> {
	const request = asking(`/token?name=${encodeURIComponent(name)}`, token, { method: 'DELETE' });
	return answerOf(() =>
		Promise.resolve(
			revokeToken({
				request,
				url: new URL(request.url),
				platform: undefined
			} as unknown as Parameters<typeof revokeToken>[0])
		)
	);
}

function invoke(name: string, token: string | null, input: unknown): Promise<RouteAnswer> {
	return reach(`/tools/${name}/invoke`, token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
}

function preview(name: string, token: string, input: unknown): Promise<RouteAnswer> {
	return reach(`/tools/${name}/target`, token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
}

function messageOf(answered: RouteAnswer): string {
	return (answered.body as { message?: string; error?: string }).message ?? '';
}

describe('a call that names nobody', () => {
	test('is refused without a bearer, and with one nobody holds', async () => {
		expect((await reach('/tools', null)).status).toBe(401);
		expect((await reach('/tools', 'ik_nobody')).status).toBe(401);
	});
});

describe('clocking attendance', () => {
	test('records with one database request and returns before its announcement completes', async () => {
		const originalFetch = globalThis.fetch;
		const databaseRequests: string[] = [];
		const backgroundWork: Promise<unknown>[] = [];
		let finishAnnouncement: (() => void) | undefined;
		globalThis.fetch = createMockFetch(async (input, options) => {
			const url = new URL(input instanceof Request ? input.url : String(input));
			if (url.pathname.startsWith('/rest/') || url.pathname.startsWith('/auth/')) {
				databaseRequests.push(url.pathname);
			}
			if (url.pathname === '/functions/v1/announce-attendance') {
				await new Promise<void>((resolve) => { finishAnnouncement = resolve; });
				return Response.json({ told: 0, reached: 0 });
			}
			return originalFetch(input, options);
		});
		try {
			const answered = await reach('/tools/attendance_add/invoke', sessionToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ input: { kind: 'clock_in' } })
			}, (work) => backgroundWork.push(work));
			expect(answered.status).toBe(200);
			expect(databaseRequests).toEqual(['/rest/v1/rpc/attendance_add']);
			expect(backgroundWork).toHaveLength(1);
			expect(answered.body).not.toHaveProperty('notified');
			const event = savedAttendanceEventSchema.parse(
				Reflect.get(Reflect.get(Object(answered.body), 'result'), 'event')
			);
			expect(event.personID).toBe(memberID);
			expect(event.kind).toBe('clock_in');
			const saved = await client.from('attendance').select('occurred_at, location').eq('id', event.id).single();
			expect(saved.error).toBeNull();
			expect(event.occurredAt).toBe(saved.data?.occurred_at);
			expect(event.location).toBe(saved.data?.location);
		} finally {
			finishAnnouncement?.();
			await Promise.all(backgroundWork);
			globalThis.fetch = originalFetch;
		}
	});

	test('keeps user-token permissions and stores the API key holder as the owner', async () => {
		expect((await invoke('attendance_add', readersToken, { kind: 'clock_out' })).status).toBe(403);
		expect((await invoke('attendance_add', departedToken, { kind: 'clock_out' })).status).toBe(403);
		expect((await invoke('attendance_add', 'invalid-session', { kind: 'clock_out' })).status).toBe(401);
		await moveAttendanceEarlier(client, memberID, 2);
		const answered = await invoke('attendance_add', holdersToken, { kind: 'clock_out' });
		expect(answered.status).toBe(200);
		expect(answered.body).toMatchObject({
			result: { status: 'added', event: { personID: memberID, kind: 'clock_out', location: null } }
		});
		expect((await invoke('attendance_add', holdersToken, { kind: 'clock_out' })).status).toBe(422);
	});

	test('is refused for the token it does not carry before its body is read', async () => {
		const answered = await reach('/tools/attendance_add/invoke', null, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { kind: 'clocking_in' } })
		});
		expect(answered.status).toBe(401);
	});

	test('serializes simultaneous clock-ins into one saved record', async () => {
		const answers = await Promise.all([
			invoke('attendance_add', sessionToken, { kind: 'clock_in' }),
			invoke('attendance_add', sessionToken, { kind: 'clock_in' })
		]);
		expect(answers.map((answer) => answer.status).sort()).toEqual([200, 422]);
		expect((await invoke('attendance_add', sessionToken, { kind: 'clock_out' })).status).toBe(200);
	});

	test('a press taken back is announced to nobody but its owner', async () => {
		const originalFetch = globalThis.fetch;
		const backgroundWork: Promise<unknown>[] = [];
		const announcements: unknown[] = [];
		globalThis.fetch = createMockFetch(async (input, options) => {
			const url = new URL(input instanceof Request ? input.url : String(input));
			if (url.pathname === '/functions/v1/announce-attendance') {
				announcements.push(
					input instanceof Request ? await input.clone().json() : JSON.parse(String(options?.body))
				);
				return Response.json({ told: 0, reached: 0 });
			}
			return originalFetch(input, options);
		});
		try {
			const answered = await reach('/tools/attendance_add/invoke', sessionToken, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ input: { kind: 'clock_in' } })
			}, (work) => backgroundWork.push(work));
			expect(answered.status).toBe(200);
			expect(answered.body).toMatchObject({ result: { status: 'removed' } });
			await Promise.all(backgroundWork);
			expect(announcements).toEqual([{ what: 'clock', colleagues: false }]);
		} finally {
			await Promise.all(backgroundWork);
			globalThis.fetch = originalFetch;
		}
	});
});

describe("a token whose owner has left", () => {
	test('is refused in the words the company machine refuses it with', async () => {
		const answered = await reach('/tools/person_list/invoke', departedToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: {} })
		});
		expect(answered.status).toBe(403);
		expect((answered.body as { message: string }).message).toBe('token owner is not active member');
	});
});

describe('a tool a runtime beside the agent answers', () => {
	test('is refused rather than carried', async () => {
		const answered = await reach('/tools/browser_open/invoke', holdersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { url: 'https://example.test/' } })
		});

		expect(answered.status).toBe(400);
		expect((answered.body as { error: string }).error).toContain('browser_open');
	});
});

describe('the catalog', () => {
	test('is answered here, and a reading token sees fewer tools than a deleting one', async () => {
		const held = await reach('/tools', holdersToken);
		const read = await reach('/tools', readersToken);

		const heldTools = (held.body as { tools: unknown[] }).tools;
		const readTools = (read.body as { tools: unknown[] }).tools;
		expect(held.status).toBe(200);
		expect(heldTools.length > 0).toBe(true);
		expect(readTools.length < heldTools.length).toBe(true);
	});

	test('answers one tool by name, and refuses a name it does not carry', async () => {
		expect((await reach('/tools/task_list', holdersToken)).status).toBe(200);
		expect((await reach('/tools/no_such_tool', holdersToken)).status).toBe(404);
	});
});

describe('a scoped task board read', () => {
	test('enforces active membership and company RLS for actual board rows', async () => {
		const boardWeek = taskWeekOfDate(new Date()).startISO;
		const other = await provisionCompany(client,
			{ name: 'Other Board Fixture', slug: `${slug}-other-board`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
			`${slug}-other-board@example.test`);
		const ownID = crypto.randomUUID();
		const foreignID = crypto.randomUUID();
		try {
			const inserted = await client.from('task').insert([
				{ id: ownID, company_id: companyID, title: 'Owned board fixture', status: 'planned' },
				{ id: foreignID, company_id: other.companyID, title: 'Other company board fixture', status: 'planned' }
			]);
			if (inserted.error) throw new Error(inserted.error.message);
			const participants = await client.from('task_participant').insert([
				{ task_id: ownID, member_id: memberID }, { task_id: foreignID, member_id: other.adminMemberID }
			]);
			if (participants.error) throw new Error(participants.error.message);
			for (const token of [readersToken, administratorsToken]) {
				const answer = await invoke('task_board_get', token, { boardWeek, scope: 'all' });
				expect(answer.status).toBe(200);
				const { result } = z.object({ result: taskBoardResultSchema }).parse(answer.body);
				expect(result.tasks.some(task => task.taskID === ownID)).toBe(true);
				expect(result.tasks.some(task => task.taskID === foreignID)).toBe(false);
			}
			expect((await invoke('task_board_get', departedToken, { boardWeek })).status).toBe(403);
			expect((await invoke('task_board_get', null, { boardWeek })).status).toBe(401);
		} finally {
			await client.from('task').delete().eq('id', ownID);
			await client.from('company').delete().eq('id', other.companyID);
		}
	}, networkHookTimeout);

	test('a read token reaches the board without changing the legacy task list contract', async () => {
		const boardWeek = taskWeekOfDate(new Date()).startISO;
		const answered = await invoke('task_board_get', readersToken, { boardWeek });
		expect(answered.status).toBe(200);
		const body = z.object({ result: taskBoardResultSchema }).parse(answered.body);
		expect(body.result.boardWeek).toBe(boardWeek);
		expect(Array.isArray(body.result.childProgress)).toBe(true);
		expect((await invoke('task_list', readersToken, { boardWeek })).status).toBe(400);
	});

	test('requires a board week and never treats missing input as a complete history read', async () => {
		const answered = await invoke('task_board_get', readersToken, {});
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.boardWeek');
	});
});

describe('a signed-in session', () => {
	test('reaches the same catalog its owner reaches, with no key issued', async () => {
		const answered = await reach('/tools', sessionToken);
		expect(answered.status).toBe(200);
		expect((answered.body as { tools: unknown[] }).tools.length > 0).toBe(true);
	});

	test('makes and revokes a token without holding one', async () => {
		const made = await mint(sessionToken, { name: 'from-a-session', permission: 'read' });
		expect(made.status).toBe(200);
		expect((made.body as { token: string }).token.startsWith('ik_')).toBe(true);
		expect((await revoke(sessionToken, 'from-a-session')).status).toBe(200);
	});
});

describe('the tokens a member holds', () => {
	test('are made, listed and revoked, and a token never outranks its maker', async () => {
		const made = (await mint(holdersToken, { name: 'made', permission: 'read' })).body as {
			name: string;
			permission: string;
			token: string;
		};
		expect(made.permission).toBe('read');

		const unnamed = (await mint(holdersToken, {})).body as { name: string };
		expect(unnamed.name).toBe('pat-1');

		const listed = (await tokens(holdersToken)).body as { tokens: { name: string }[] };
		const names = listed.tokens.map((token) => token.name);
		expect(names).toContain('made');
		expect(names).toContain('pat-1');

		expect((await mint(readersToken, { name: 'escalation', permission: 'delete' })).status).toBe(403);
		expect((await mint(holdersToken, { name: 'holder' })).status).toBe(409);

		expect((await revoke(holdersToken, 'holder')).status).toBe(409);
		expect((await revoke(holdersToken, 'never-existed')).status).toBe(404);
		expect((await revoke(holdersToken, made.name)).status).toBe(200);
		expect((await reach('/tools', made.token)).status).toBe(401);
	});
});

describe('a token that has lived its lifetime', () => {
	const dayMilliseconds = 24 * 60 * 60 * 1000;

	test('lives ninety days unless the maker asks for another whole number of days up to a year', async () => {
		const madeAt = Date.now();
		const made = (await mint(holdersToken, { name: 'ninety-days' })).body as { expiresAt: string };
		expect(Date.parse(made.expiresAt) - madeAt).toBeGreaterThanOrEqual(90 * dayMilliseconds - 60_000);
		expect(Date.parse(made.expiresAt) - madeAt).toBeLessThanOrEqual(90 * dayMilliseconds + 60_000);

		const week = (await mint(holdersToken, { name: 'a-week', expiresInDays: 7 })).body as { expiresAt: string };
		expect(Date.parse(week.expiresAt) - madeAt).toBeLessThanOrEqual(7 * dayMilliseconds + 60_000);

		expect((await mint(holdersToken, { name: 'none', expiresInDays: 0 })).status).toBe(400);
		expect((await mint(holdersToken, { name: 'forever', expiresInDays: 366 })).status).toBe(400);
		expect((await mint(holdersToken, { name: 'half', expiresInDays: 1.5 })).status).toBe(400);
	});

	test('is refused, and making one by the same name renews it', async () => {
		const longAgo = new Date(Date.now() - 2 * dayMilliseconds);
		const expired = await issuePersonalAccessToken(client, memberID, 'expired', 'read', 1, longAgo);

		const refused = await reach('/tools', expired);
		expect(refused.status).toBe(401);
		expect(messageOf(refused)).toBe('this token has expired; make another by the same name to renew it');

		const renewed = (await mint(holdersToken, { name: 'expired', permission: 'read' })).body as { token: string };
		expect((await reach('/tools', renewed.token)).status).toBe(200);
	});

	test('is listed with when it expires and when it was last used', async () => {
		const used = await issuePersonalAccessToken(client, memberID, 'used', 'read');
		expect((await reach('/tools', used)).status).toBe(200);

		const listed = (await tokens(holdersToken)).body as {
			tokens: { name: string; expiresAt: string; lastUsedAt: string | null }[];
		};
		const entry = listed.tokens.find((token) => token.name === 'used');
		expect(Date.parse(entry?.expiresAt ?? '')).toBeGreaterThan(Date.now());
		expect(Date.parse(entry?.lastUsedAt ?? '')).toBeGreaterThan(Date.now() - 60_000);
	});
});

describe('a tool whose rows live in the record', () => {
	test('answers every call one token makes at once, the way a home screen widget reads what it shows', async () => {
		const email = `${slug}-widget@example.test`;
		const widgetMemberID = await addMember(client, companyID, email);
		const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
		await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', widgetMemberID);
		const widgetToken = await issuePersonalAccessToken(client, widgetMemberID, 'ios-widget', 'write');

		const answered = await Promise.all([
			invoke('attendance_list', widgetToken, {}),
			invoke('company_settings_get', widgetToken, {})
		]);
		expect(answered.map((answer) => [answer.status, messageOf(answer)])).toEqual([
			[200, ''],
			[200, '']
		]);
	});

	test('is answered here, without a gateway to any company machine', async () => {
		const answered = await reach('/tools/person_list/invoke', holdersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: {} })
		});
		expect(answered.status).toBe(200);
		expect((answered.body as { tool: string }).tool).toBe('person_list');
	});

	// The route reads a blank as a field left out wherever the field refuses one,
	// so a clearing this side of it has to survive that read to reach the record.
	test('takes a task out from under its parent, which is a blank the route must not swallow', async () => {
		const parent = await invoke('task_add', holdersToken, { title: '경로를 지나는 상위 업무' });
		const child = await invoke('task_add', holdersToken, {
			title: '경로를 지나는 하위 업무',
			parentTaskHint: resultOf(parent).taskID as string
		});
		expect(resultOf(child).parentTaskID).toBe(resultOf(parent).taskID as string);

		const released = await invoke('task_update', holdersToken, {
			taskHint: resultOf(child).taskID as string,
			parentTaskHint: ''
		});
		expect(released.status).toBe(200);
		expect(resultOf(released).parentTaskID).toBe('');
	});

	test('is reachable even when the model is never shown it, which is how the board writes labels', async () => {
		const answered = await invoke('task_vocabulary_set', administratorsToken, {
			businesses: [{ name: '사업하나', color: '#2563eb' }],
			types: [{ name: '개선' }]
		});
		expect(answered.status).toBe(200);
		expect((resultOf(answered).businesses as { name: string }[]).map((label) => label.name)).toEqual(['사업하나']);
	});

	test('is refused by rung before it runs', async () => {
		const answered = await reach('/tools/task_delete/invoke', readersToken, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ input: { taskHint: 'anything' } })
		});
		expect(answered.status).toBe(403);
	});
});

describe('a tool the company machine runs', () => {
	test('is refused with no gateway configured rather than answered here', async () => {
		const answered = await invoke('message_send', holdersToken, {
			targetType: 'channel',
			channelName: 'town-square',
			message: 'hello'
		});
		expect(answered.status).toBe(503);
	});

	test('is refused here when its input does not fit, before anything is carried', async () => {
		const answered = await invoke('message_send', holdersToken, {
			targetType: 'channel',
			conversationID: 'c1',
			message: 'hello'
		});
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.conversationID');
	});
});

describe('looking at what a destructive call would touch', () => {
	test('answers no target for a tool that has nothing to look at', async () => {
		const answered = await preview('task_list', holdersToken, {});
		expect(answered.status).toBe(200);
		expect(answered.body).toEqual({ tool: 'task_list', target: null });
	});

	test('answers no target for a tool the company machine answers', async () => {
		const answered = await preview('message_delete', holdersToken, { messageIDs: ['a-message'] });
		expect(answered.status).toBe(200);
		expect(answered.body).toEqual({ tool: 'message_delete', target: null });
	});

	test('is refused by rung before it resolves anything', async () => {
		const answered = await preview('task_delete', readersToken, { taskHint: 'anything' });
		expect(answered.status).toBe(403);
	});

	test('reads the input against the same schema the call is held to', async () => {
		const answered = await preview('task_delete', holdersToken, { colour: 'red' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.colour');
	});
});

describe('input the catalog does not publish', () => {
	test('is refused for a value outside the enum, naming the field', async () => {
		const answered = await invoke('task_add', holdersToken, { title: 'a task', size: 'huge' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.size');
	});

	test('is refused for a field the strict object does not carry', async () => {
		const answered = await invoke('task_list', holdersToken, { colour: 'red' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.colour');
	});

	test('is refused for a required field left out', async () => {
		const answered = await invoke('task_add', holdersToken, { size: 'M' });
		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.title');
	});

	test('lets an input the catalog does publish through to the record', async () => {
		const answered = await invoke('task_list', holdersToken, { scope: 'self' });
		expect(answered.status).toBe(200);
	});

	test('answers the labels a company registered in the colours it gave them', async () => {
		await client
			.from('company')
			.update({ task_vocabulary: { businesses: [{ name: '영업', color: '#2563eb' }], types: [] } })
			.eq('id', companyID);

		const answered = await invoke('task_list', holdersToken, { scope: 'self' });
		const labels = (answered.body as { result: { registeredLabels: { businesses: unknown[] } } }).result
			.registeredLabels;

		expect(labels.businesses).toEqual([{ name: '영업', color: '#2563eb' }]);
	});
});

function resultOf(answered: RouteAnswer): Record<string, unknown> {
	return (answered.body as { result: Record<string, unknown> }).result;
}

function descriptorOf(answered: RouteAnswer): Record<string, unknown> {
	return answered.body as Record<string, unknown>;
}

describe('the CRM this company keeps', () => {
	test('is opened through the route: the words it runs in, an organization, a person and a deal', async () => {
		const words = await invoke('crm_vocabulary_set', administratorsToken, {
			organizationTypes: [{ id: 'customer', name: '고객' }],
			pipelines: [{ id: 'partnership', name: '파트너십' }]
		});
		expect(words.status).toBe(200);

		const organization = await invoke('crm_organization_add', holdersToken, {
			name: '샘플상사',
			types: ['customer'],
			importance: 'high'
		});
		expect(organization.status).toBe(200);
		expect(resultOf(organization).name).toBe('샘플상사');

		const contact = await invoke('crm_contact_add', holdersToken, {
			organizationHint: '샘플상사',
			name: '박예시',
			email: 'yesi@example.com'
		});
		expect(contact.status).toBe(200);

		const deal = await invoke('crm_opportunity_add', holdersToken, {
			organizationHint: '샘플상사',
			title: '샘플상사 도입',
			contactHint: 'yesi',
			amountMinor: 18000000,
			currencyCode: 'KRW'
		});
		expect(deal.status).toBe(200);
		expect(resultOf(deal).stage).toBe('waiting');
		expect(resultOf(deal).pipeline).toBe('partnership');
	});

	test('is read by a signed-in session and by a token alike', async () => {
		const throughASession = await invoke('crm_organization_list', sessionToken, { query: '샘플상사' });
		const throughAToken = await invoke('crm_opportunity_list', holdersToken, { stage: 'waiting' });

		expect((resultOf(throughASession).organizations as unknown[]).length).toBe(1);
		expect((resultOf(throughAToken).opportunities as unknown[]).length).toBe(1);
		expect((resultOf(await invoke('crm_contact_list', sessionToken, {})).contacts as unknown[]).length).toBe(1);
	});

	test('changes a deal, then moves it, and the closing move settles what it was worth', async () => {
		const changed = await invoke('crm_opportunity_update', holdersToken, {
			opportunityHint: '샘플상사 도입',
			amountMinor: 20000000,
			expectedCloseDate: '2026-09-30'
		});
		expect(changed.status).toBe(200);
		expect(resultOf(changed).amountMinor).toBe(20000000);
		expect(resultOf(changed).expectedCloseTimeZone).toBe('Asia/Seoul');

		const moved = await invoke('crm_opportunity_move', holdersToken, {
			opportunityHint: '샘플상사 도입',
			stage: 'review',
			position: 1
		});
		expect(resultOf(moved).stage).toBe('review');

		const closed = await invoke('crm_opportunity_move', holdersToken, {
			opportunityHint: '샘플상사 도입',
			stage: 'done'
		});
		expect(resultOf(closed).stage).toBe('done');
		expect(resultOf(closed).baseAmountMinor).toBe(20000000);
		expect(resultOf(closed).baseCurrencyCode).toBe('KRW');
	});

	test('says which of its tools pause for approval and which do not', async () => {
		const move = await reach('/tools/crm_opportunity_move', holdersToken);
		const update = await reach('/tools/crm_opportunity_update', holdersToken);
		const archive = await reach('/tools/crm_opportunity_archive', holdersToken);

		expect(descriptorOf(move).requiresApproval).toBe(true);
		expect(descriptorOf(archive).requiresApproval).toBe(true);
		expect(descriptorOf(update).requiresApproval).toBeUndefined();
		expect(descriptorOf(archive).sideEffectClass).toBe('destructive');
	});

	test('refuses a stage the record does not name, before anything is carried', async () => {
		const answered = await invoke('crm_opportunity_move', holdersToken, {
			opportunityHint: '샘플상사 도입',
			stage: 'negotiating'
		});

		expect(answered.status).toBe(400);
		expect(messageOf(answered)).toContain('input.stage');
	});

	test('refuses a hint that names nothing, with the candidates to name instead', async () => {
		const answered = await invoke('crm_organization_update', holdersToken, {
			organizationHint: '없는회사',
			importance: 'low'
		});

		expect(answered.status).toBe(409);
		expect((answered.body as { errorCode: string }).errorCode).toBe('crm_organization_not_found');
	});

	test('refuses a token that may only read, and one whose owner has left', async () => {
		const readOnly = await invoke('crm_organization_add', readersToken, { name: '읽기만' });
		expect(readOnly.status).toBe(403);

		const departed = await invoke('crm_organization_list', departedToken, {});
		expect(departed.status).toBe(403);
		expect(messageOf(departed)).toBe('token owner is not active member');
	});

	test('records an activity and lists it back against the deal', async () => {
		const recorded = await invoke('crm_activity_save', holdersToken, {
			organizationHint: '샘플상사',
			opportunityHint: '샘플상사 도입',
			title: '킥오프 미팅',
			kind: 'meeting',
			note: '요구사항을 들었다',
			occurredAt: '2026-09-02T09:00:00+09:00'
		});
		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).content).toBe('요구사항을 들었다');

		const listed = await invoke('crm_activity_list', holdersToken, { opportunityHint: '샘플상사 도입' });
		const kept = resultOf(listed).activities as { title: string; kind: string }[];
		expect(kept.map((activity) => activity.title)).toContain('킥오프 미팅');
		expect(kept.map((activity) => activity.kind)).toContain('stage_change');
	});

	test('says what an archive would take away, then takes it away', async () => {
		const target = await preview('crm_opportunity_archive', holdersToken, {
			opportunityHint: '샘플상사 도입'
		});
		expect(target.status).toBe(200);
		expect((target.body as { target: { title: string } }).target.title).toBe('샘플상사 도입');

		const archived = await invoke('crm_opportunity_archive', holdersToken, {
			opportunityHint: '샘플상사 도입'
		});
		expect(archived.status).toBe(200);

		const left = await invoke('crm_opportunity_list', holdersToken, {});
		expect((resultOf(left).opportunities as unknown[]).length).toBe(0);
		expect(
			(resultOf(await invoke('crm_opportunity_list', holdersToken, { includeArchived: true }))
				.opportunities as unknown[]).length
		).toBe(1);
	});
});

function pictureCall(token: string, picture?: File): Promise<RouteAnswer> {
	const carried = new FormData();
	if (picture) carried.set('file', picture);
	return reach('/company/profile-image', token, {
		method: 'POST',
		...(picture ? { body: carried } : { headers: { 'Content-Type': 'application/json' }, body: '{}' })
	});
}

describe("the company's picture", () => {
	const png = () => new File([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], 'logo.png', { type: 'image/png' });

	test('is refused to a token that may only read', async () => {
		const answered = await pictureCall(readersToken, png());

		expect(answered.status).toBe(403);
	});

	test('is refused to somebody who does not administer the company', async () => {
		const answered = await pictureCall(holdersToken, png());

		expect(answered.status).toBe(403);
		expect(messageOf(answered)).toContain('administrator');
	});

	test('is refused when the call carries no picture, and when it carries something else', async () => {
		expect((await pictureCall(administratorsToken)).status).toBe(400);
		const document = new File([new Uint8Array([37, 80, 68, 70])], 'terms.pdf', { type: 'application/pdf' });
		expect((await pictureCall(administratorsToken, document)).status).toBe(400);
	});

	// The bucket is private, so what comes back is an address somebody signed
	// rather than the path it was stored at, and company_settings_get answers
	// the same picture from the same row.
	test('is kept for an administrator, and answered as an address a browser can show', async () => {
		const answered = await pictureCall(administratorsToken, png());

		expect(answered.status).toBe(200);
		const kept = (answered.body as { profileImageURL: string }).profileImageURL;
		expect(kept).toContain('/storage/v1/');

		const settings = await invoke('company_settings_get', administratorsToken, {});
		expect((settings.body as { result: { profileImageURL: string } }).result.profileImageURL).toContain('/storage/v1/');
	});

	test('is taken down by an administrator, and nothing is answered in its place', async () => {
		const answered = await reach('/company/profile-image', administratorsToken, { method: 'DELETE' });

		expect(answered.status).toBe(200);
		expect((answered.body as { profileImageURL: string | null }).profileImageURL).toBeNull();

		const settings = await invoke('company_settings_get', administratorsToken, {});
		expect((settings.body as { result: { profileImageURL: string | null } }).result.profileImageURL).toBeNull();
	});
});

function memberPictureCall(token: string, picture?: File): Promise<RouteAnswer> {
	const carried = new FormData();
	if (picture) carried.set('file', picture);
	return reach('/member/profile-image', token, {
		method: 'POST',
		...(picture ? { body: carried } : { headers: { 'Content-Type': 'application/json' }, body: '{}' })
	});
}

describe("a member's own picture", () => {
	const drawn = () => new File([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10, 1])], 'drawn.png', { type: 'image/png' });
	const messenger = () => new File([new Uint8Array([255, 216, 255, 224, 2])], 'messenger.jpg', { type: 'image/jpeg' });

	test('is refused to a token that may only read', async () => {
		expect((await memberPictureCall(readersToken, drawn())).status).toBe(403);
	});

	test('is refused when the call carries no picture, and when it carries something that is not a picture', async () => {
		expect((await memberPictureCall(holdersToken)).status).toBe(400);
		const document = new File([new Uint8Array([37, 80, 68, 70])], 'terms.pdf', { type: 'application/pdf' });
		expect((await memberPictureCall(holdersToken, document)).status).toBe(400);
	});

	test('is kept, left alone when the same picture comes again, and replaced by the messenger picture', async () => {
		expect((await memberPictureCall(holdersToken, drawn())).body).toEqual({ kept: true });
		expect((await memberPictureCall(holdersToken, drawn())).body).toEqual({ kept: false });
		expect((await memberPictureCall(holdersToken, messenger())).body).toEqual({ kept: true });
	});
});

type Operation = { path: string; method: string };
type PathsOfDocument = Record<string, Record<string, unknown> | undefined>;

function documentedOperations(): Operation[] {
	const paths = (createOpenApiDocument('en') as { paths: PathsOfDocument }).paths;
	const operations: Operation[] = [];
	for (const [path, methods] of Object.entries(paths)) {
		for (const method of Object.keys(methods ?? {})) operations.push({ path, method });
	}
	return operations;
}

// A documented path carries {name} where a real call carries a tool. The routes
// only have to recognise the shape, so any name they can resolve will do.
function reachDocumented(operation: Operation, revocableName: string): Promise<RouteAnswer> {
	const method = operation.method.toUpperCase();
	const carriesBody = method === 'POST' || method === 'PUT' || method === 'PATCH';
	const path = operation.path.replace('{name}', 'task_list');
	if (path === '/mcp') return speakMCP(holdersToken);
	if (path === '/tokens') return tokens(holdersToken);
	if (operation.path === '/data-room/links/{linkID}') {
		const linkID = dataRoomLinkID;
		const request = asking(`/data-room/links/${linkID}`, sessionToken, { method });
		const route = method === 'POST' ? unlockDataRoomLink : readDataRoomLink;
		return answerOf(() => Promise.resolve(route({ request, url: new URL(request.url),
			params: { linkID }, platform: undefined, cookies: { get: () => dataRoomLinkCookie, set: () => {} }, getClientAddress: () => '127.0.0.1' })));
	}
	if (path === '/data-room/{companyID}') {
		const request = asking(`/data-room/${companyID}`, sessionToken);
		return answerOf(() => Promise.resolve(readDataRoom({ request, url: new URL(request.url),
			params: { companyID }, platform: undefined } as Parameters<typeof readDataRoom>[0])));
	}
	if (operation.path === '/data-room/invitations/{shareID}') {
		const shareID = crypto.randomUUID();
		const request = asking(`/data-room/invitations/${shareID}`, sessionToken, { method: 'POST' });
		return answerOf(() => Promise.resolve(acceptDataRoom({ request,
			params: { shareID }, platform: undefined } as Parameters<typeof acceptDataRoom>[0])));
	}
	if (operation.path === '/data-room/invitations/{shareID}/send') {
		const request = asking(`/data-room/invitations/${dataRoomInvitationID}/send`, administratorSession, { method: 'POST' });
		return answerOf(() => Promise.resolve(sendDataRoomInvitation({ request, url: new URL(request.url),
			params: { shareID: dataRoomInvitationID }, platform: undefined } as Parameters<typeof sendDataRoomInvitation>[0])));
	}
	if (path === '/token' && method === 'POST') return mint(holdersToken, {});
	if (path === '/token' && method === 'DELETE') return revoke(holdersToken, revocableName);
	return reach(path, holdersToken, {
		method,
		headers: { 'Content-Type': 'application/json' },
		...(carriesBody ? { body: '{}' } : {})
	});
}

function speakMCP(token: string): Promise<RouteAnswer> {
	const request = asking('/mcp', token, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' },
		body: JSON.stringify({
			jsonrpc: '2.0',
			id: 1,
			method: 'initialize',
			params: { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'conformance', version: '1' } }
		})
	});
	return answerOf(() =>
		Promise.resolve(
			reachMCP({
				request,
				url: new URL(request.url),
				params: {},
				platform: undefined
			} as unknown as Parameters<typeof reachMCP>[0])
		)
	);
}

describe('the documented endpoints', () => {
	test('are more than a handful, so an empty document cannot pass this', () => {
		expect(documentedOperations().length > 30).toBe(true);
	});

	test('are all reachable, and none answers as if it carried no token', async () => {
		const wrong: string[] = [];
		for (const [ordinal, operation] of documentedOperations().entries()) {
			// Only the revoking operation spends a token, so only it needs one minted.
			const revocableName = `conformance-${ordinal}`;
			if (operation.path === '/token' && operation.method.toUpperCase() === 'DELETE') {
				await mint(holdersToken, { name: revocableName, permission: 'read' });
			}
			const answered = await reachDocumented(operation, revocableName);
			if ([401, 404, 405].includes(answered.status)) {
				wrong.push(`${operation.method.toUpperCase()} ${operation.path} -> ${answered.status}`);
			}
		}
		expect(wrong).toEqual([]);
	}, 60_000);

	test('cover every path these routes answer', () => {
		const documented = new Set(
			documentedOperations().map((operation) => `${operation.method} ${operation.path}`)
		);
		const served = [
			'get /tools',
			'get /tools/{name}',
			'post /mcp',
			'post /tools/{name}/invoke',
			'post /tools/{name}/target',
			'get /tokens',
			'post /token',
			'delete /token',
			'post /files',
			'post /company/profile-image',
			'delete /company/profile-image',
			'post /member/profile-image',
			'post /agent/messages',
			'get /agent/replies',
			'get /data-room/{companyID}',
			'post /data-room/invitations/{shareID}',
			'post /data-room/invitations/{shareID}/send',
			'get /data-room/links/{linkID}',
			'post /data-room/links/{linkID}'
		];
		expect(served.filter((operation) => !documented.has(operation))).toEqual([]);
	});
});

function posting(path: string, token: string, body: unknown): Request {
	return new Request(`https://space.example.test${path}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

function eventOf(request: Request) {
	return { request, url: new URL(request.url), platform: undefined };
}

function resetting(token: string, body: unknown): Promise<RouteAnswer> {
	const event = eventOf(posting('/api/member/password-reset', token, body));
	return answerOf(() => Promise.resolve(resetPassword(event as unknown as Parameters<typeof resetPassword>[0])));
}

function removing(token: string, body: unknown): Promise<RouteAnswer> {
	const event = eventOf(posting('/api/member/remove', token, body));
	return answerOf(() => Promise.resolve(removeMember(event as unknown as Parameters<typeof removeMember>[0])));
}

function inviting(token: string, body: unknown): Promise<RouteAnswer> {
	const event = eventOf(posting('/api/member/invite', token, body));
	return answerOf(() => Promise.resolve(invitePerson(event as unknown as Parameters<typeof invitePerson>[0])));
}

function issuingTheFeed(token: string): Promise<RouteAnswer> {
	const event = eventOf(posting('/api/calendar/subscription', token, {}));
	return answerOf(() =>
		Promise.resolve(issueCalendarFeed(event as unknown as Parameters<typeof issueCalendarFeed>[0]))
	);
}

describe('a personal access token does not administer the company', () => {
	test("an administrator's token resets nobody's password, whatever it may otherwise do", async () => {
		const { data: administrator } = await client
			.from('member')
			.select('id')
			.eq('company_id', companyID)
			.eq('is_admin', true)
			.single();
		const readOnly = await issuePersonalAccessToken(client, administrator!.id, 'administrator-reads', 'read');

		for (const token of [readOnly, administratorsToken]) {
			const answer = await resetting(token, { memberID });
			expect(answer.status).toBe(403);
		}
	});

	test("a read-only token does not issue the company's calendar feed", async () => {
		const { data: administrator } = await client
			.from('member')
			.select('id')
			.eq('company_id', companyID)
			.eq('is_admin', true)
			.single();
		const readOnly = await issuePersonalAccessToken(client, administrator!.id, 'administrator-reads-calendar', 'read');

		const answer = await issuingTheFeed(readOnly);
		expect(answer.status).toBe(403);
	});
});

describe('an administrator and another administrator', () => {
	let secondSession = '';
	let firstAdministratorID = '';

	beforeAll(async () => {
		const { data: first } = await client
			.from('member')
			.select('id')
			.eq('company_id', companyID)
			.eq('email', `${slug}-admin@example.test`)
			.single();
		firstAdministratorID = first!.id;
		const secondID = await addMember(client, companyID, `${slug}-second-admin@example.test`);
		await client.from('member').update({ is_admin: true, status: 'active' }).eq('id', secondID);
		secondSession = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, secondID)).accessToken;
	}, networkHookTimeout);

	test('resets a colleague\'s password, and refuses another administrator\'s', async () => {
		expect((await resetting(secondSession, { memberID })).status).toBe(200);
		expect((await resetting(secondSession, { memberID: firstAdministratorID })).status).toBe(403);
	});

	test('removes another administrator', async () => {
		const departingID = await addMember(client, companyID, `${slug}-departing-admin@example.test`);
		await client.from('member').update({ is_admin: true, status: 'active' }).eq('id', departingID);

		expect((await removing(secondSession, { memberID: departingID })).status).toBe(200);
		const { data: departing } = await client.from('member').select('status').eq('id', departingID).single();
		expect(departing!.status).toBe('withdrawn');
	});

	test('invites a colleague as an administrator', async () => {
		const answer = await inviting(secondSession, {
			email: `${slug}-promoted@example.test`,
			name: 'Promoted',
			isAdmin: true
		});
		expect(answer.status).toBe(200);
		const { data: promoted } = await client
			.from('member')
			.select('is_admin')
			.eq('email', `${slug}-promoted@example.test`)
			.single();
		expect(promoted).toEqual({ is_admin: true });
	});
});

describe('a refused picture write', () => {
	test("takes down no picture another row still shows", async () => {
		const { data: top } = await client
			.from('member')
			.select('id')
			.eq('company_id', companyID)
			.eq('email', `${slug}-admin@example.test`)
			.single();
		await client.from('member').update({ status: 'active' }).eq('id', top!.id);
		const administratorSession = (await sessionForMember({ projectURL, serviceRoleKey, signingKey }, top!.id))
			.accessToken;
		const picture = () => {
			const form = new FormData();
			form.set('file', new File([new Uint8Array([137, 80, 78, 71, 1, 2, 3, 4])], 'logo.png', { type: 'image/png' }));
			return { method: 'POST', body: form };
		};

		const shown = await reach('/company/profile-image', administratorSession, picture());
		expect(shown.status).toBe(200);
		const { data: company } = await client.from('company').select('profile_image').eq('id', companyID).single();

		const refused = await reach('/company/profile-image', sessionToken, picture());
		expect(refused.status).toBe(403);

		const { data: stillThere, error } = await client.storage.from('asset').download(company!.profile_image);
		expect(error).toBeNull();
		expect(stillThere?.size).toBeGreaterThan(0);

		await client.from('company').update({ profile_image: null }).eq('id', companyID);
		await client.storage.from('asset').remove([company!.profile_image]);
	});
});
