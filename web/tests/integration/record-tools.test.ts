import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { asMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';
import catalog from '../../../pkg/capabilityprotocol/generated/capability-tools.json';
import { dayIn } from '../../src/lib/server/public-api/record/days';
import { capabilityToolResultSchema } from '../../src/lib/server/public-api/catalog/tools';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord, recordRunsTheTool, toolsTheRecordRuns, recordToolsWithoutAnImplementation } =
	await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `record-tools-${Date.now()}`;
const companyDay = dayIn('Asia/Seoul', new Date());

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

async function run(name: string, input: Record<string, unknown> = {}) {
	const answered = await runToolOverTheRecord(caller, sampleID, name, input, new Date());
	if (answered.status === 200) holdToTheContract(name, (answered.body as { result: unknown }).result);
	return answered;
}

function holdToTheContract(name: string, result: unknown): void {
	const schema = capabilityToolResultSchema(name);
	if (!schema) throw new Error(`${name} publishes no result contract to hold its answer to`);
	const parsed = schema.safeParse(result);
	const refused = parsed.success
		? []
		: parsed.error.issues.map((issue) => `${['result', ...issue.path.map(String)].join('.')}: ${issue.message}`);
	expect({ tool: name, refused }).toEqual({ tool: name, refused: [] });
}

function resultOf(answer: { status: number; body: unknown }): Record<string, unknown> {
	expect(answer.status).toBe(200);
	return (answer.body as { result: Record<string, unknown> }).result;
}

describe('which tools run over the record', () => {
	test('are exactly the ones whose descriptor says the record answers them', () => {
		const answeredByTheRecord = catalog.tools
			.filter((tool) => tool.answeredBy === 'record')
			.map((tool) => tool.name)
			.sort();

		expect(answeredByTheRecord).toHaveLength(19);
		expect([...toolsTheRecordRuns()].sort()).toEqual(answeredByTheRecord);
	});

	test('all have somewhere to run', () => {
		expect(recordToolsWithoutAnImplementation()).toEqual([]);
	});

	test('leave the tools the company machine runs to it', () => {
		for (const tool of catalog.tools) {
			expect(recordRunsTheTool(tool.name)).toBe(tool.answeredBy === 'record');
		}
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
		expect(made.participantNames).toEqual(['이샘플']);
		expect(made.business).toBe('');
		expect(made.status).toBe('planned');
		expect(made.content).toBe('분기 보고서 초안');
		expect((made.participantPresentations as { mention: string }[])[0].mention).toBe('@이샘플');
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
		expect(changed.content).toBe('월간 회고 정리');
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
		expect(made.participantNames).toEqual(['박예시']);
	});

	test('refuses a label the company never registered, and says what it has', async () => {
		const refused = await run('task_add', { title: '엉뚱한 라벨', business: '마케팅' });
		expect(refused.status).toBe(409);
		expect((refused.body as { registered: string[] }).registered).toEqual(['영업', '개발']);
	});

	test('refuses a hint several tasks answer to, and names them with their ids', async () => {
		await run('task_add', { title: '주차 안내 하나' });
		await run('task_add', { title: '주차 안내 둘' });
		const refused = await run('task_update', { taskHint: '주차 안내', status: 'completed' });
		const refusal = refused.body as {
			errorCode: string;
			failureStage: string;
			retryable: boolean;
			safeRetry: boolean;
			candidates: { id: string; label: string }[];
		};
		expect(refused.status).toBe(409);
		expect(refusal.errorCode).toBe('interaction_required');
		expect(refusal.failureStage).toBe('target_resolution');
		expect(refusal.retryable).toBe(true);
		expect(refusal.safeRetry).toBe(true);
		expect(refusal.candidates.map((one) => one.label).sort()).toEqual(['주차 안내 둘', '주차 안내 하나']);
		expect(refusal.candidates.every((one) => one.id !== '')).toBe(true);
	});

	test('offers the nearest people when a name is a character off, and resolves nobody', async () => {
		const refused = await run('task_add', { title: '가까운 이름', participantPersonHints: ['박예시연'] });
		const refusal = refused.body as {
			errorCode: string;
			candidates: { id: string; label: string; email: string; handle: string }[];
		};
		expect(refused.status).toBe(409);
		expect(refusal.errorCode).toBe('interaction_required');
		expect(refusal.candidates.map((one) => one.label)).toContain('박예시');

		// The retry the refusal invites has to work on the value it handed back.
		const offered = refusal.candidates.find((one) => one.label === '박예시');
		expect(offered?.handle).toBe(`@${slug}-example`);
		const named = resultOf(
			await run('task_add', { title: '가까운 이름', participantPersonHints: [offered!.handle] })
		);
		expect(named.participantNames).toEqual(['박예시']);
	});

	test('refuses a name nothing comes close to as a participant, not as a person', async () => {
		const refused = await run('task_add', { title: '없는 사람', participantPersonHints: ['최견본'] });
		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode: string }).errorCode).toBe('task_participant_not_found');
	});

	test('carries a second call within ten minutes into the first rather than adding a copy', async () => {
		const first = resultOf(await run('task_add', { title: '중복될 업무' }));
		const second = resultOf(await run('task_add', { title: '중복될 업무', size: 'L' }));
		expect(second.taskID).toBe(first.taskID);
		expect(second.size).toBe('L');

		const listed = resultOf(await run('task_list', { scope: 'all', weekFrom: -400, weekTo: 400 }));
		const copies = (listed.tasks as { content: string }[]).filter((task) => task.content === '중복될 업무');
		expect(copies.length).toBe(1);
	});

	test('refuses a rule of the record as something to fix, not as a permission', async () => {
		await run('task_add', { title: '이미 끝난 기간', startsAt: '2020-01-06', endsAt: '2020-01-10' });
		const refused = await run('task_update', { taskHint: '이미 끝난 기간', status: 'in_progress' });
		expect(refused.status).toBe(422);
		expect(String((refused.body as { error: string }).error)).toContain('end');
	});
});

function titlesOf(listed: Record<string, unknown>): string[] {
	return (listed.tasks as { content: string }[]).map((task) => task.content);
}

describe('task_list', () => {
	test('answers the caller’s own tasks by default and everyone’s on request', async () => {
		const mine = resultOf(await run('task_list'));
		const everyone = resultOf(await run('task_list', { scope: 'all' }));
		expect(mine.scope).toBe('person');
		expect(everyone.scope).toBe('everyone');
		expect((everyone.count as number) >= (mine.count as number)).toBe(true);
	});

	test('keeps only the weeks asked for', async () => {
		await run('task_add', {
			title: '작년에 끝낸 일',
			status: 'completed',
			startsAt: '2020-01-06',
			endsAt: '2020-01-10'
		});
		const fixedWeek = resultOf(await run('task_list', { scope: 'all', weekFrom: 0, weekTo: 0 }));
		expect(titlesOf(fixedWeek)).not.toContain('작년에 끝낸 일');

		const wide = resultOf(await run('task_list', { scope: 'all', weekFrom: -400, weekTo: 400 }));
		expect(titlesOf(wide)).toContain('작년에 끝낸 일');
	});

	test('keeps only the status asked for, and stops at the limit', async () => {
		const running = resultOf(await run('task_list', { scope: 'all', status: 'in_progress' }));
		expect((running.tasks as { status: string }[]).every((task) => task.status === 'in_progress')).toBe(true);

		const capped = resultOf(await run('task_list', { scope: 'all', limit: 1 }));
		expect(capped.count).toBe(1);
	});

	test('answers this week when no week was asked for', async () => {
		const asked = resultOf(await run('task_list', { scope: 'all' }));
		expect(titlesOf(asked)).not.toContain('작년에 끝낸 일');
	});

	test('rolls planned work that has not started forward into the week being read', async () => {
		await run('task_add', { title: '지난주에 잡아둔 계획', startsAt: dayAround(-21), endsAt: dayAround(-14) });
		const thisWeek = resultOf(await run('task_list', { scope: 'all' }));
		expect((thisWeek.tasks as { content: string }[]).map((task) => task.content)).toContain(
			'지난주에 잡아둔 계획'
		);
	});

	test('matches a query against more than the title, ignoring case and spacing', async () => {
		const byBusiness = resultOf(await run('task_list', { scope: 'all', weekFrom: -400, weekTo: 400, query: '개 발' }));
		const businesses = (byBusiness.tasks as { business: string }[]).map((task) => task.business);
		expect(businesses.length > 0).toBe(true);
		expect(businesses.every((business) => business === '개발')).toBe(true);
	});
});

describe('an event written through the record', () => {
	test('is read back within the week it falls in', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '주간 회의',
				startsAt: `${companyDay}T10:00:00+09:00`,
				endsAt: `${companyDay}T11:00:00+09:00`,
				location: '회의실'
			})
		);
		expect(made.location).toBe('회의실');
		expect((made.participants as { name: string }[]).map((one) => one.name)).toEqual(['이샘플']);
		expect(made.updatedAt).not.toBe('');

		const listed = resultOf(await run('event_list', { weekFrom: 0, weekTo: 0 }));
		expect((listed.events as { title: string }[]).map((event) => event.title)).toContain('주간 회의');
	});

	test('takes everyone attending as everyone in the company', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '전사 공지',
				startsAt: `${companyDay}T14:00:00+09:00`,
				endsAt: `${companyDay}T15:00:00+09:00`,
				everyoneAttends: true
			})
		);
		expect((made.participants as unknown[]).length >= 3).toBe(true);
	});

	test('keeps what an update did not name', async () => {
		const changed = resultOf(await run('event_update', { eventHint: '주간 회의', title: '주간 회의 (연장)' }));
		expect(changed.location).toBe('회의실');
		expect(changed.startsAt).not.toBe('');
	});

	test('lets only the first of two updates from one base land', async () => {
		const base = resultOf(
			await run('event_add', {
				title: '분기 리뷰',
				startsAt: `${dayAround(1)}T10:00:00+09:00`,
				endsAt: `${dayAround(1)}T11:00:00+09:00`
			})
		);
		const readVersion = base.updatedAt as string;

		const first = resultOf(
			await run('event_update', {
				eventHint: '분기 리뷰',
				expectedUpdatedAt: readVersion,
				location: '회의실 A'
			})
		);
		expect(first.location).toBe('회의실 A');
		expect(first.updatedAt).not.toBe(readVersion);

		const second = await run('event_update', {
			eventHint: '분기 리뷰',
			expectedUpdatedAt: readVersion,
			location: '회의실 B'
		});
		expect(second.status).toBe(409);
		expect(second.body).toEqual({
			error: 'this event changed since the version named here was read',
			errorCode: 'calendar_event_version_conflict',
			failureStage: 'resolution',
			retryable: true,
			safeRetry: false,
			updatedAt: first.updatedAt
		});

		const held = resultOf(await run('event_update', { eventHint: '분기 리뷰', note: '' }));
		expect(held.location).toBe('회의실 A');
	});

	test('refuses a delete that names a version the event has moved past', async () => {
		const refused = await run('event_delete', {
			eventHint: '분기 리뷰',
			expectedUpdatedAt: '2020-01-01T00:00:00.000Z'
		});
		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode: string }).errorCode).toBe('calendar_event_version_conflict');

		const still = resultOf(await run('event_list', { query: '분기 리뷰' }));
		expect((still.events as { title: string }[]).map((event) => event.title)).toEqual(['분기 리뷰']);
	});

	test('refuses an event that ends before it starts', async () => {
		const refused = await run('event_add', {
			title: '거꾸로',
			startsAt: `${companyDay}T17:00:00+09:00`,
			endsAt: `${companyDay}T16:00:00+09:00`
		});
		expect(refused.status).toBe(400);
	});
});

describe('deleting', () => {
	test('takes away a task the caller stands alone on', async () => {
		const gone = resultOf(await run('task_delete', { taskHint: '분기 보고서 초안' }));
		expect(gone.deleted).toBe(true);
		const left = resultOf(await run('task_list', { scope: 'all' }));
		expect((left.tasks as { content: string }[]).map((task) => task.content)).not.toContain('분기 보고서 초안');
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
