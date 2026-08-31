import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { asMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord, recordRunsTheTool } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `record-tools-${Date.now()}`;
const now = new Date('2026-08-26T03:00:00.000Z');

// A task the record will accept as running has to be running: the company's own
// clock decides that, not the fixed instant these week windows are read from.
function dayAround(days: number): string {
	return new Date(Date.now() + days * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

let companyID = '';
let sampleID = '';
let exampleID = '';
let caller: ReturnType<typeof asMember>;

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Record Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	exampleID = await addMember(client, companyID, `${slug}-example@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '박예시' }).eq('id', exampleID);
	await client
		.from('company')
		.update({ task_vocabulary: { businesses: [{ name: '영업' }, { name: '개발' }], types: [{ name: '문서' }] } })
		.eq('id', companyID);

	const { data: account } = await client.auth.admin.createUser({
		email: `${slug}-sample@example.test`,
		email_confirm: true
	});
	await client.from('member').update({ user_id: account.user!.id }).eq('id', sampleID);

	const session = await sessionForMember({ projectURL, serviceRoleKey }, sampleID);
	caller = asMember({ projectURL, publishableKey }, session.accessToken);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function run(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(caller, sampleID, name, input, now);
}

function resultOf(answer: { status: number; body: unknown }): Record<string, unknown> {
	expect(answer.status).toBe(200);
	return (answer.body as { result: Record<string, unknown> }).result;
}

describe('which tools run over the record', () => {
	test('are the nine whose rows live there, and nothing else', () => {
		for (const name of [
			'task_add',
			'task_update',
			'task_list',
			'task_delete',
			'event_add',
			'event_update',
			'event_list',
			'event_delete',
			'person_list'
		]) {
			expect(recordRunsTheTool(name)).toBe(true);
		}
		expect(recordRunsTheTool('message_send')).toBe(false);
		expect(recordRunsTheTool('site_serve')).toBe(false);
	});
});

describe('person_list', () => {
	test('answers the company from the record itself', async () => {
		const result = resultOf(await run('person_list'));
		const names = (result.people as { name: string }[]).map((person) => person.name);
		expect(names).toContain('이샘플');
		expect(names).toContain('박예시');
	});
});

describe('a task written through the record', () => {
	test('belongs to the caller when nobody was named', async () => {
		const made = resultOf(await run('task_add', { title: '분기 보고서 초안', size: 'M' }));
		expect(made.participants).toEqual(['이샘플']);
		expect(made.business).toBe('영업');
		expect(made.status).toBe('planned');
	});

	test('keeps what an update did not name', async () => {
		const made = resultOf(
			await run('task_add', {
				title: '월간 회고 정리',
				size: 'S',
				business: '개발',
				type: '문서',
				startsAt: dayAround(-7),
				endsAt: dayAround(7)
			})
		);

		const changed = resultOf(await run('task_update', { taskHint: '월간 회고 정리', status: 'in_progress' }));
		expect(changed.taskID).toBe(made.taskID);
		expect(changed.status).toBe('in_progress');
		expect(changed.title).toBe('월간 회고 정리');
		expect(changed.size).toBe('S');
		expect(changed.business).toBe('개발');
		expect(changed.type).toBe('문서');
		expect(changed.startDate).toBe(dayAround(-7));
		expect(changed.endDate).toBe(dayAround(7));
	});

	test('names the people a caller named, by part of a name', async () => {
		const made = resultOf(
			await run('task_add', { title: '채용 공고 검토', participantPersonHints: ['예시'] })
		);
		expect(made.participants).toEqual(['박예시']);
	});

	test('refuses a label the company never registered, and says what it has', async () => {
		const refused = await run('task_add', { title: '엉뚱한 라벨', business: '마케팅' });
		expect(refused.status).toBe(409);
		expect((refused.body as { registered: string[] }).registered).toEqual(['영업', '개발']);
	});

	test('refuses a hint several tasks answer to, and names them', async () => {
		await run('task_add', { title: '주차 안내 하나' });
		await run('task_add', { title: '주차 안내 둘' });
		const refused = await run('task_update', { taskHint: '주차 안내', status: 'completed' });
		expect(refused.status).toBe(409);
		expect([...(refused.body as { candidates: string[] }).candidates].sort()).toEqual([
			'주차 안내 둘',
			'주차 안내 하나'
		]);
	});

	test('refuses a rule of the record as something to fix, not as a permission', async () => {
		await run('task_add', { title: '이미 끝난 기간', startsAt: '2020-01-06', endsAt: '2020-01-10' });
		const refused = await run('task_update', { taskHint: '이미 끝난 기간', status: 'in_progress' });
		expect(refused.status).toBe(422);
		expect(String((refused.body as { error: string }).error)).toContain('end');
	});
});

describe('task_list', () => {
	test('answers the caller’s own tasks by default and everyone’s on request', async () => {
		const mine = resultOf(await run('task_list'));
		const everyone = resultOf(await run('task_list', { scope: 'all' }));
		expect(mine.scope).toBe('person');
		expect(everyone.scope).toBe('everyone');
		expect((everyone.count as number) >= (mine.count as number)).toBe(true);
	});

	test('keeps only the weeks asked for', async () => {
		const fixedWeek = resultOf(await run('task_list', { scope: 'all', weekFrom: 0, weekTo: 0 }));
		const titles = (fixedWeek.tasks as { title: string }[]).map((task) => task.title);
		expect(titles).not.toContain('이미 끝난 기간');

		const wide = resultOf(await run('task_list', { scope: 'all', weekFrom: -400, weekTo: 400 }));
		expect((wide.tasks as { title: string }[]).map((task) => task.title)).toContain('이미 끝난 기간');
	});

	test('keeps only the status asked for, and stops at the limit', async () => {
		const running = resultOf(await run('task_list', { scope: 'all', status: 'in_progress' }));
		expect((running.tasks as { status: string }[]).every((task) => task.status === 'in_progress')).toBe(true);

		const capped = resultOf(await run('task_list', { scope: 'all', limit: 1 }));
		expect(capped.count).toBe(1);
	});
});

describe('an event written through the record', () => {
	test('is read back within the week it falls in', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '주간 회의',
				startsAt: '2026-08-26T10:00:00+09:00',
				endsAt: '2026-08-26T11:00:00+09:00',
				location: '회의실'
			})
		);
		expect(made.location).toBe('회의실');
		expect(made.participants).toEqual(['이샘플']);

		const listed = resultOf(await run('event_list', { weekFrom: 0, weekTo: 0 }));
		expect((listed.events as { title: string }[]).map((event) => event.title)).toContain('주간 회의');
	});

	test('takes everyone attending as everyone in the company', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '전사 공지',
				startsAt: '2026-08-27T10:00:00+09:00',
				endsAt: '2026-08-27T11:00:00+09:00',
				everyoneAttends: true
			})
		);
		expect((made.participants as string[]).length >= 3).toBe(true);
	});

	test('keeps what an update did not name', async () => {
		const changed = resultOf(await run('event_update', { eventHint: '주간 회의', title: '주간 회의 (연장)' }));
		expect(changed.location).toBe('회의실');
		expect(changed.startsAt).not.toBe('');
	});

	test('refuses an event that ends before it starts', async () => {
		const refused = await run('event_add', {
			title: '거꾸로',
			startsAt: '2026-08-28T11:00:00+09:00',
			endsAt: '2026-08-28T10:00:00+09:00'
		});
		expect(refused.status).toBe(400);
	});
});

describe('deleting', () => {
	test('takes away a task the caller stands alone on', async () => {
		const gone = resultOf(await run('task_delete', { taskHint: '분기 보고서 초안' }));
		expect(gone.deleted).toBe(true);
		const left = resultOf(await run('task_list', { scope: 'all' }));
		expect((left.tasks as { title: string }[]).map((task) => task.title)).not.toContain('분기 보고서 초안');
	});

	test('refuses to take away a task that is somebody else’s', async () => {
		const refused = await run('task_delete', { taskHint: '채용 공고 검토' });
		expect(refused.status).toBe(403);
	});

	test('takes an event away too', async () => {
		const gone = resultOf(await run('event_delete', { eventHint: '전사 공지' }));
		expect(gone.deleted).toBe(true);
	});

	test('refuses to delete something it cannot find', async () => {
		const refused = await run('task_delete', { taskHint: '없는 업무' });
		expect(refused.status).toBe(409);
	});
});
