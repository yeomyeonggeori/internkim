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

const {
	previewToolOverTheRecord,
	runToolOverTheRecord,
	recordRunsTheTool,
	toolsTheRecordRuns,
	recordToolsWithoutAnImplementation
} = await import('../../src/lib/server/public-api/record');

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
	const answered = await runToolOverTheRecord(caller, client, sampleID, name, input, new Date());
	if (answered.status === 200) holdToTheContract(name, (answered.body as { result: unknown }).result);
	return answered;
}

function previewOf(name: string, input: Record<string, unknown>) {
	return previewToolOverTheRecord(caller, client, sampleID, name, input, new Date());
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

		expect(answeredByTheRecord.length).toBeGreaterThan(0);
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

	test('moves the days of a task it already holds', async () => {
		const made = resultOf(
			await run('task_add', { title: '분기 계획 정리', startsAt: dayAround(-3), endsAt: dayAround(3) })
		);

		const moved = await run('task_update', { taskHint: '분기 계획 정리', endsAt: dayAround(10) });
		expect(moved.status).toBe(200);
		expect(resultOf(moved).taskID).toBe(made.taskID);
		expect(resultOf(moved).endDate).toBe(dayAround(10));
	});

	test('takes a status the record then derives away from', async () => {
		await run('task_add', { title: '이사회 자료 준비', startsAt: dayAround(5), endsAt: dayAround(9) });

		const moved = await run('task_update', {
			taskHint: '이사회 자료 준비',
			status: 'planned',
			startsAt: dayAround(-1)
		});
		expect(moved.status).toBe(200);
		expect(resultOf(moved).status).toBe('in_progress');
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

	test('answers every week there is when asked, whatever the week range says', async () => {
		const everything = resultOf(await run('task_list', { scope: 'all', everyWeek: true, weekFrom: 0, weekTo: 0 }));
		expect(titlesOf(everything)).toContain('작년에 끝낸 일');
	});

	test('names the labels this company registered, with the colours it painted them', async () => {
		const listed = resultOf(await run('task_list', { scope: 'all' }));
		const labels = listed.registeredLabels as { businesses: { name: string }[]; etcBusinessColor?: string };
		expect(labels.businesses.map((label) => label.name)).toContain('개발');
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

	test('keeps a reminder named in minutes, and gives it up when told none', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '알림 있는 회의',
				startsAt: '2026-08-28T10:00:00+09:00',
				endsAt: '2026-08-28T11:00:00+09:00',
				notifyMinutesBefore: 30
			})
		);
		expect(made.notifyMinutesBefore).toBe(30);

		const moved = resultOf(await run('event_update', { eventHint: made.eventID, notifyMinutesBefore: 60 }));
		expect(moved.notifyMinutesBefore).toBe(60);

		const untouched = resultOf(await run('event_update', { eventHint: made.eventID, title: '알림 있는 회의' }));
		expect(untouched.notifyMinutesBefore).toBe(60);

		const cleared = resultOf(await run('event_update', { eventHint: made.eventID, notifyMinutesBefore: 0 }));
		expect(cleared.notifyMinutesBefore).toBeUndefined();
	});

	test('leaves an event everyone attends with no attendee list', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '전사 공지',
				startsAt: `${companyDay}T14:00:00+09:00`,
				endsAt: `${companyDay}T15:00:00+09:00`,
				everyoneAttends: true
			})
		);
		expect(made.participants).toEqual([]);
	});

	test('answers the hours where the company is, not in UTC', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '자리 표기 확인',
				startsAt: `${companyDay}T14:00:00+09:00`,
				endsAt: `${companyDay}T15:30:00+09:00`
			})
		);
		expect(made.startsAt).toBe(`${companyDay}T14:00:00+09:00`);
		expect(made.endsAt).toBe(`${companyDay}T15:30:00+09:00`);
	});

	test('covers the whole of the company days a whole-day event names', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '종일 워크숍',
				startsAt: '2026-08-24',
				endsAt: '2026-08-25',
				isWholeDay: true
			})
		);
		expect(made.isWholeDay).toBe(true);
		expect(made.startsAt).toBe('2026-08-24T00:00:00+09:00');
		expect(made.endsAt).toBe('2026-08-26T00:00:00+09:00');

		const listed = resultOf(await run('event_list', { startsAt: '2026-08-24', endsAt: '2026-08-25' }));
		expect((listed.events as { title: string }[]).map((event) => event.title)).toContain('종일 워크숍');
	});

	test('stands an event asked of somebody else at requested', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '남에게 부탁한 일정',
				startsAt: `${companyDay}T16:00:00+09:00`,
				endsAt: `${companyDay}T16:30:00+09:00`,
				participantPersonHints: ['박예시']
			})
		);
		const listed = resultOf(await run('event_list', { query: '남에게 부탁한' }));
		expect((listed.events as unknown[]).length).toBe(1);
		expect((made.participants as { name: string }[]).map((one) => one.name)).toEqual(['박예시']);
	});

	test('refuses a second copy of one event, and names the one it already holds', async () => {
		const first = resultOf(
			await run('event_add', {
				title: '한 번만 있어야 할 회의',
				startsAt: `${companyDay}T18:00:00+09:00`,
				endsAt: `${companyDay}T19:00:00+09:00`
			})
		);
		const refused = await run('event_add', {
			title: '한 번만 있어야 할 회의',
			startsAt: `${companyDay}T18:00:00+09:00`,
			endsAt: `${companyDay}T19:00:00+09:00`
		});
		const refusal = refused.body as {
			errorCode: string;
			retryable: boolean;
			safeRetry: boolean;
			eventID: string;
		};
		expect(refused.status).toBe(409);
		expect(refusal.errorCode).toBe('calendar_event_duplicate');
		expect(refusal.retryable).toBe(false);
		expect(refusal.safeRetry).toBe(false);
		expect(refusal.eventID).toBe(first.eventID as string);
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

	test('answers what is coming when no window was asked for', async () => {
		const upcoming = resultOf(await run('event_list'));
		const titles = (upcoming.events as { title: string }[]).map((event) => event.title);
		expect(titles).toContain('주간 회의 (연장)');
	});

	test('matches a query against the note and the place, whatever the case', async () => {
		const byPlace = resultOf(await run('event_list', { query: '회의실' }));
		expect((byPlace.events as { title: string }[]).map((event) => event.title)).toContain(
			'주간 회의 (연장)'
		);
	});

	test('fills the other end of a window given only one', async () => {
		const fromOnly = resultOf(await run('event_list', { startsAt: `${companyDay}T00:00:00+09:00` }));
		expect((fromOnly.events as unknown[]).length > 0).toBe(true);
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

describe('looking at what a delete would touch', () => {
	test('names the event the delete then deletes, and writes nothing', async () => {
		const made = resultOf(
			await run('event_add', {
				title: '미리 볼 일정',
				startsAt: `${companyDay}T20:00:00+09:00`,
				endsAt: `${companyDay}T21:00:00+09:00`
			})
		);

		const previewed = await previewOf('event_delete', { eventHint: '미리 볼 일정' });
		expect(previewed.status).toBe(200);
		const target = (previewed.body as { target: { inputField: string; id: string; title: string } }).target;
		expect(target.id).toBe(made.eventID as string);
		expect(target.title).toBe('미리 볼 일정');
		expect(target.inputField).toBe('eventHint');

		const stillThere = resultOf(await run('event_list', { query: '미리 볼 일정' }));
		expect((stillThere.events as unknown[]).length).toBe(1);

		const gone = resultOf(await run('event_delete', { eventHint: target.id }));
		expect(gone.eventID).toBe(target.id);
	});

	test('answers a hint several tasks hold with the candidates the call would answer with', async () => {
		await run('task_add', { title: '미리보기 후보 하나' });
		await run('task_add', { title: '미리보기 후보 둘' });

		const previewed = await previewOf('task_delete', { taskHint: '미리보기 후보' });
		const invoked = await run('task_delete', { taskHint: '미리보기 후보' });

		expect(previewed.status).toBe(409);
		expect(previewed.body).toEqual(invoked.body);
	});

	test('answers no target for a destructive tool with nothing to resolve ahead', async () => {
		const previewed = await previewOf('leave_delete', { leaveHint: 'anything' });
		expect(previewed.status).toBe(200);
		expect((previewed.body as { target: unknown }).target).toBeNull();
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

describe('what the board reads a task by', () => {
	test('is its parent, who asked for it, and when it was made', async () => {
		const parent = resultOf(await run('task_add', { title: '묶음이 되는 일' }));
		await run('task_add', { title: '묶음에 딸린 일', parentTaskHint: parent.taskID as string });

		const listed = resultOf(await run('task_list', { scope: 'all', everyWeek: true }));
		const child = (listed.tasks as Record<string, string>[]).find((task) => task.content === '묶음에 딸린 일');
		expect(child?.parentTaskID).toBe(parent.taskID as string);
		expect(child?.requesterID).toBe(sampleID);
		expect(Number.isNaN(Date.parse(child?.createdAt ?? ''))).toBe(false);
	});
});

describe('a task put under another', () => {
	test('goes under the one named, and comes back out when none is', async () => {
		const parent = resultOf(await run('task_add', { title: '상위가 되는 일' }));
		const child = resultOf(await run('task_add', { title: '상위를 갖는 일' }));

		const under = resultOf(await run('task_update', {
			taskHint: child.taskID as string,
			parentTaskHint: parent.taskID as string
		}));
		expect(under.parentTaskID).toBe(parent.taskID as string);

		const alone = resultOf(await run('task_update', { taskHint: child.taskID as string, parentTaskHint: '' }));
		expect(alone.parentTaskID).toBe('');
	});

	test('changes nothing else about the task it moves', async () => {
		const parent = resultOf(await run('task_add', { title: '아무것도 바꾸지 않을 상위' }));
		const child = resultOf(
			await run('task_add', { title: '그대로 남을 일', size: 'L', business: '개발', endsAt: dayAround(3) })
		);

		const moved = resultOf(await run('task_update', {
			taskHint: child.taskID as string,
			parentTaskHint: parent.taskID as string
		}));
		expect(moved).toMatchObject({ content: '그대로 남을 일', size: 'L', business: '개발' });
		expect(moved.endDate).toBe(child.endDate as string);
	});

	test('takes several at once, or none of them when one cannot go', async () => {
		const parent = resultOf(await run('task_add', { title: '여럿을 받는 일' }));
		const first = resultOf(await run('task_add', { title: '함께 들어갈 일 하나' }));
		const second = resultOf(await run('task_add', { title: '함께 들어갈 일 둘' }));

		const grouped = resultOf(await run('task_update', {
			taskHint: parent.taskID as string,
			childTaskHints: [first.taskID as string, second.taskID as string]
		}));
		expect(grouped.taskID).toBe(parent.taskID as string);

		const listed = resultOf(await run('task_list', { scope: 'all', everyWeek: true }));
		const parents = (listed.tasks as Record<string, string>[])
			.filter((task) => [first.taskID, second.taskID].includes(task.taskID))
			.map((task) => task.parentTaskID);
		expect(parents).toEqual([parent.taskID as string, parent.taskID as string]);

		const refused = await run('task_update', {
			taskHint: parent.taskID as string,
			childTaskHints: [first.taskID as string]
		});
		expect(refused.status).toBe(422);
	});
});

describe('a date taken off a task', () => {
	test('is cleared by an empty string rather than left where it was', async () => {
		const made = resultOf(await run('task_add', { title: '기한을 지울 일', endsAt: dayAround(4) }));
		expect(made.endDate).toBe(dayAround(4));

		const cleared = resultOf(await run('task_update', { taskHint: made.taskID as string, endsAt: '' }));
		expect(cleared.endDate).toBe('');
	});
});

describe('task_vocabulary_set', () => {
	beforeAll(async () => {
		await client.from('member').update({ is_admin: true }).eq('id', sampleID);
	});

	afterAll(async () => {
		await client.from('member').update({ is_admin: false }).eq('id', sampleID);
	});

	test('writes the labels the company files work under and reads them back', async () => {
		const written = resultOf(await run('task_vocabulary_set', {
			businesses: [{ name: '영업' }, { name: '개발', color: '#2563eb' }, { name: '지원' }],
			types: [{ name: '문서' }],
			etcBusinessColor: '#94a3b8'
		}));
		expect((written.businesses as { name: string }[]).map((label) => label.name)).toEqual(['영업', '개발', '지원']);
		expect(written.etcBusinessColor).toBe('#94a3b8');

		const listed = resultOf(await run('task_list', { scope: 'all' }));
		const labels = listed.registeredLabels as { businesses: { name: string; color?: string }[]; etcBusinessColor?: string };
		expect(labels.businesses.find((label) => label.name === '개발')?.color).toBe('#2563eb');
		expect(labels.etcBusinessColor).toBe('#94a3b8');
	});

	test('refuses to drop a label a task is still filed under', async () => {
		await run('task_add', { title: '개발로 남아 있을 일', business: '개발' });

		const refused = await run('task_vocabulary_set', { businesses: [{ name: '영업' }], types: [{ name: '문서' }] });
		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode: string }).errorCode).toBe('task_label_in_use');
	});
});
