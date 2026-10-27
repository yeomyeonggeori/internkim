import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from '../support/supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';
import { dayIn, dayShifted, weekWindow } from '../../src/lib/server/public-api/record/days';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { runToolOverTheRecord, previewToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `crm-tools-${Date.now()}`;
const otherSlug = `crm-outsider-${Date.now()}`;
const now = new Date();

let companyID = '';
let adminID = '';
let sampleID = '';
let outsiderCompanyID = '';
let outsiderID = '';
let admin: ReturnType<typeof asMember>;
let sample: ReturnType<typeof asMember>;
let outsider: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'CRM Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);

	const outsiderCompany = await provisionCompany(
		client,
		{ name: 'CRM Outsider', slug: otherSlug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${otherSlug}-admin@example.test`
	);
	outsiderCompanyID = outsiderCompany.companyID;
	outsiderID = outsiderCompany.adminMemberID;
	outsider = await signedInMember(outsiderID, `${otherSlug}-admin@example.test`);

	await client
		.from('company')
		.update({
			task_vocabulary: { businesses: [{ name: '영업', color: '#2563eb' }], types: [{ name: 'meeting' }] }
		})
		.eq('id', companyID);

	await asAdmin('crm_vocabulary_set', {
		organizationTypes: [{ id: 'customer', name: '고객' }],
		pipelines: [{ id: 'partnership', name: '파트너십', direction: 'outbound' }]
	});
}, networkHookTimeout);

afterAll(async () => {
	for (const id of [companyID, outsiderCompanyID]) {
		if (!id) continue;
		const { data: members } = await client.from('member').select('user_id').eq('company_id', id);
		await client.from('company').delete().eq('id', id);
		for (const member of members ?? []) {
			if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
		}
	}
}, networkHookTimeout);

async function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(admin, client, adminID, name, input, now, leavesTaskLabelsUndecided));
}

async function asSample(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(sample, client, sampleID, name, input, now, leavesTaskLabelsUndecided));
}

async function asOutsider(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(outsider, client, outsiderID, name, input, now, leavesTaskLabelsUndecided));
}

function resultOf(answer: { body: unknown }): Record<string, unknown> {
	return (answer.body as { result: Record<string, unknown> }).result;
}

function errorOf(answer: { body: unknown }): string {
	return (answer.body as { error?: string }).error ?? '';
}

function errorCodeOf(answer: { body: unknown }): string {
	return (answer.body as { errorCode?: string }).errorCode ?? '';
}

function candidatesOf(answer: { body: unknown }): { id: string; label: string }[] {
	return (answer.body as { candidates?: { id: string; label: string }[] }).candidates ?? [];
}

function organizationsOf(answer: { body: unknown }): Record<string, unknown>[] {
	return resultOf(answer).organizations as Record<string, unknown>[];
}

function opportunitiesOf(answer: { body: unknown }): Record<string, unknown>[] {
	return resultOf(answer).opportunities as Record<string, unknown>[];
}

function activitiesOf(answer: { body: unknown }): Record<string, unknown>[] {
	return resultOf(answer).activities as Record<string, unknown>[];
}

describe('the organizations a company deals with', () => {
	test('are added, listed and changed by anybody who works here', async () => {
		const added = await asSample('crm_organization_add', {
			name: 'ABC상사',
			types: ['customer'],
			importance: 'high',
			address: '서울',
			ownerPersonHint: '이샘플'
		});

		expect(added.status).toBe(200);
		expect(resultOf(added).name).toBe('ABC상사');
		expect(resultOf(added).ownerPersonID).toBe(sampleID);
		expect(resultOf(added).types).toEqual(['customer']);

		const listed = await asSample('crm_organization_list', { query: 'abc' });
		expect(organizationsOf(listed)).toHaveLength(1);

		const changed = await asSample('crm_organization_update', {
			organizationHint: 'ABC상사',
			importance: 'medium',
			address: ''
		});
		expect(resultOf(changed).importance).toBe('medium');
		expect(resultOf(changed).address).toBe('');
	});

	test('are named by a part only one of them carries, and several ask which', async () => {
		await asSample('crm_organization_add', { name: 'ABC엔지니어링' });

		const resolved = await asSample('crm_organization_update', {
			organizationHint: '엔지니어링',
			importance: 'low'
		});
		expect(resolved.status).toBe(200);
		expect(resultOf(resolved).name).toBe('ABC엔지니어링');

		const ambiguous = await asSample('crm_organization_update', { organizationHint: 'ABC', importance: 'low' });
		expect(ambiguous.status).toBe(409);
		expect(errorCodeOf(ambiguous)).toBe('interaction_required');
		expect(candidatesOf(ambiguous).map((candidate) => candidate.label).sort()).toEqual([
			'ABC상사',
			'ABC엔지니어링'
		]);
	});

	test('are not reached by somebody who works at another company', async () => {
		const refused = await asOutsider('crm_organization_archive', { organizationHint: 'ABC상사' });

		expect(refused.status).toBe(409);
		expect(errorCodeOf(refused)).toBe('crm_organization_not_found');
		expect(organizationsOf(await asSample('crm_organization_list', { query: 'ABC상사' }))).toHaveLength(1);
	});

	test('leave the screens when archived, and say what a call would touch first', async () => {
		const previewed = await previewToolOverTheRecord(
			sample,
			client,
			sampleID,
			'crm_organization_archive',
			{ organizationHint: 'ABC엔지니어링' },
			now
		);
		expect((previewed.body as { target: { title: string } }).target.title).toBe('ABC엔지니어링');

		const archived = await asSample('crm_organization_archive', { organizationHint: 'ABC엔지니어링' });
		expect(resultOf(archived).name).toBe('ABC엔지니어링');
		expect(organizationsOf(await asSample('crm_organization_list', {}))).toHaveLength(1);
		expect(organizationsOf(await asSample('crm_organization_list', { includeArchived: true }))).toHaveLength(2);
	});
});

describe('the people at those organizations', () => {
	test('are added under one and named by their email local part', async () => {
		const added = await asSample('crm_contact_add', {
			organizationHint: 'ABC상사',
			name: '박예시',
			email: 'yesi@example.com',
			role: '구매팀장'
		});

		expect(added.status).toBe(200);
		expect(resultOf(added).role).toBe('구매팀장');

		const changed = await asSample('crm_contact_update', { contactHint: 'yesi', phoneNumber: '02-000-0000' });
		expect(resultOf(changed).phoneNumber).toBe('02-000-0000');

		const listed = await asSample('crm_contact_list', { organizationHint: 'ABC상사' });
		expect((resultOf(listed).contacts as unknown[]).length).toBe(1);
	});

	test('are refused when nobody could reach them, on the way in and on the way out', async () => {
		const refused = await asSample('crm_contact_add', {
			organizationHint: 'ABC상사',
			name: '연락처 없는 사람'
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('email address or a phone number');

		const emptied = await asSample('crm_contact_update', {
			contactHint: 'yesi',
			email: '',
			phoneNumber: ''
		});

		expect(emptied.status).toBe(400);
		expect(errorOf(emptied)).toContain('email address or a phone number');
	});
});

describe('the deals a company is working on', () => {
	test('open at an open stage and refuse to open at a closing one', async () => {
		const refused = await asSample('crm_opportunity_add', {
			organizationHint: 'ABC상사',
			title: '이미 이긴 건',
			stage: 'done'
		});
		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toContain('crm_opportunity_move');

		const opened = await asSample('crm_opportunity_add', {
			organizationHint: 'ABC상사',
			title: 'ABC상사 도입',
			contactHint: 'yesi',
			amount: { amount: 18000000, currency: 'KRW' },
			business: '영업',
			expectedCloseDate: '2026-09-30'
		});
		expect(opened.status).toBe(200);
		expect(resultOf(opened).stage).toBe('waiting');
		expect(resultOf(opened).pipeline).toBe('partnership');
		expect(resultOf(opened).expectedCloseTimeZone).toBe('Asia/Seoul');
		expect(resultOf(opened).amount).toEqual({ amount: 18000000, currency: 'KRW' });
	});

	test('are changed field by field, keeping what the call does not name', async () => {
		const changed = await asSample('crm_opportunity_update', {
			opportunityHint: 'ABC상사 도입',
			importance: 'medium',
			description: '보안 검토가 남았다'
		});

		expect(changed.status).toBe(200);
		expect(resultOf(changed).importance).toBe('medium');
		expect(resultOf(changed).description).toBe('보안 검토가 남았다');
		expect(resultOf(changed).amount).toEqual({ amount: 18000000, currency: 'KRW' });
		expect(resultOf(changed).contactID).not.toBe('');
	});

	test('are listed by the stage they stand at and the organization they are with', async () => {
		expect(opportunitiesOf(await asSample('crm_opportunity_list', { stage: 'waiting' }))).toHaveLength(1);
		expect(opportunitiesOf(await asSample('crm_opportunity_list', { stage: 'done' }))).toHaveLength(0);
		expect(
			opportunitiesOf(await asSample('crm_opportunity_list', { organizationHint: 'ABC상사' }))
		).toHaveLength(1);
	});

	test('move stage by stage, and a move into lost needs a reason', async () => {
		const moved = await asSample('crm_opportunity_move', {
			opportunityHint: 'ABC상사 도입',
			stage: 'review',
			position: 2
		});
		expect(moved.status).toBe(200);
		expect(resultOf(moved).stage).toBe('review');
		expect(resultOf(moved).stagePosition).toBe(2);

		const refused = await asSample('crm_opportunity_move', { opportunityHint: 'ABC상사 도입', stage: 'lost' });
		expect(refused.status).toBe(422);
		expect(errorOf(refused)).toContain('reason');
	});

	test('settle what they were worth when they close', async () => {
		const settled = await asSample('crm_opportunity_move', { opportunityHint: 'ABC상사 도입', stage: 'done' });

		expect(resultOf(settled).stage).toBe('done');
		expect(resultOf(settled).settledAmount).toEqual({ amount: 18000000, currency: 'KRW' });
	});

	test('are reordered without moving stage, which leaves the day they last moved alone', async () => {
		const stood = opportunitiesOf(await asSample('crm_opportunity_list', {}))[0];
		const reordered = await asSample('crm_opportunity_move', { opportunityHint: 'ABC상사 도입', position: 7 });

		expect(resultOf(reordered).stagePosition).toBe(7);
		expect(resultOf(reordered).stage).toBe('done');
		expect(resultOf(reordered).stageChangedAt).toBe(stood.stageChangedAt);
	});

	test('are seen by whoever is responsible for them, and handed to someone out of the giver\'s sight', async () => {
		const colleagueEmail = `${slug}-colleague@example.test`;
		const colleagueID = await addMember(client, companyID, colleagueEmail);
		await client.from('member').update({ name: '박예시' }).eq('id', colleagueID);
		const colleague = await signedInMember(colleagueID, colleagueEmail);
		const asColleague = async (name: string, input: Record<string, unknown> = {}) =>
			heldToTheContract(name, await runToolOverTheRecord(colleague, client, colleagueID, name, input, now, leavesTaskLabelsUndecided));

		expect(opportunitiesOf(await asColleague('crm_opportunity_list', {}))).toHaveLength(0);
		expect(organizationsOf(await asColleague('crm_organization_list', {})).length).toBeGreaterThan(0);

		await asAdmin('crm_opportunity_add', { organizationHint: 'ABC상사', title: '견본 이관 건', ownerPersonHint: '이샘플' });
		const handed = await asSample('crm_opportunity_update', {
			opportunityHint: '견본 이관 건',
			ownerPersonHint: '박예시',
			description: '박예시가 이어받는다'
		});

		expect(handed.status).toBe(200);
		expect(resultOf(handed).ownerPersonID).toBe(colleagueID);
		expect(resultOf(handed).description).toBe('박예시가 이어받는다');
		expect(opportunitiesOf(await asSample('crm_opportunity_list', {})).map((deal) => deal.title)).not.toContain('견본 이관 건');
		expect(opportunitiesOf(await asColleague('crm_opportunity_list', {})).map((deal) => deal.title)).toEqual(['견본 이관 건']);
	});
});

describe('the words a company runs its CRM in', () => {
	test('are answered with the stages the product fixes', async () => {
		const answered = await asSample('crm_vocabulary_get');

		expect(resultOf(answered).pipelines).toEqual([
			{ id: 'partnership', name: '파트너십', direction: 'outbound' }
		]);
		expect((resultOf(answered).stages as { stage: string }[]).map((stage) => stage.stage)).toEqual([
			'waiting',
			'in_progress',
			'review',
			'done',
			'on_hold',
			'lost'
		]);
	});

	test('keep a pipeline a deal still runs in', async () => {
		const refused = await asAdmin('crm_vocabulary_set', {
			organizationTypes: [{ id: 'customer', name: '고객' }],
			pipelines: []
		});

		expect(refused.status).toBe(409);
		expect(errorCodeOf(refused)).toBe('crm_definition_in_use');
	});

	test('are an administrator to write', async () => {
		const refused = await asSample('crm_vocabulary_set', {
			organizationTypes: [],
			pipelines: [{ id: 'partnership', name: '파트너십' }]
		});

		expect(refused.status).toBe(403);
	});
});

describe('the work recorded against a deal', () => {
	test('is an ordinary task that names the deal it belongs to', async () => {
		const opportunity = opportunitiesOf(await asSample('crm_opportunity_list', {}))[0];
		const added = await asSample('task_add', {
			title: 'ABC상사 견적서 보내기',
			opportunityHint: 'ABC상사 도입'
		});

		expect(added.status).toBe(200);
		expect(resultOf(added).opportunityID).toBe(opportunity.opportunityID);
		expect(resultOf(added).organizationID).toBe(opportunity.organizationID);
	});

	test('is listed by the deal it belongs to, whichever week it falls in', async () => {
		const listed = await asSample('task_list', { opportunityHint: 'ABC상사 도입', scope: 'all' });

		expect((resultOf(listed).tasks as { content: string }[]).map((task) => task.content)).toContain(
			'ABC상사 견적서 보내기'
		);
	});

	test('is taken off the deal by an empty hint', async () => {
		const taken = await asSample('task_update', {
			taskHint: 'ABC상사 견적서 보내기',
			organizationHint: ''
		});

		expect(taken.status).toBe(200);
		expect(resultOf(taken).opportunityID).toBe('');
		expect(resultOf(taken).organizationID).toBe('');
	});
});

describe('the activities recorded against a deal', () => {
	test('carry every colleague taking part, not one assignee', async () => {
		await asSample('crm_organization_add', { name: '참여자상사', types: ['customer'] });

		const recorded = await asSample('crm_activity_save', {
			organizationHint: '참여자상사',
			title: '둘이 함께 나간 미팅',
			kind: 'meeting',
			participantPersonHints: ['이샘플', '최견본']
		});

		expect(recorded.status).toBe(200);
		// task_participant carries no ordering column, so membership is the contract, not order.
		expect([...(resultOf(recorded).participantIDs as string[])].sort()).toEqual([sampleID, adminID].sort());
	});

	test('accept the older single ownerPersonHint as one participant', async () => {
		await asSample('crm_organization_add', { name: '단독상사', types: ['customer'] });

		const recorded = await asSample('crm_activity_save', {
			organizationHint: '단독상사',
			title: '혼자 처리한 통화',
			kind: 'meeting',
			ownerPersonHint: '이샘플'
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).participantIDs).toEqual([sampleID]);
	});

	test('are written through the same task writer the task tools use', async () => {
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			opportunityHint: 'ABC상사 도입',
			contactHint: 'yesi',
			title: '킥오프 미팅',
			kind: 'meeting',
			business: '영업',
			note: '요구사항을 들었다',
			startsAt: '2026-09-02T09:00:00+09:00'
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).content).toBe('요구사항을 들었다');
		expect(resultOf(recorded).kind).toBe('meeting');
		expect(resultOf(recorded).isEvent).toBe(false);
		expect(resultOf(recorded).isWholeDay).toBe(false);
		expect(new Date(resultOf(recorded).occurredAt as string).toISOString()).toBe(
			'2026-09-02T00:00:00.000Z'
		);
		expect(resultOf(recorded).startsAt).toBe('2026-09-02T09:00:00+09:00');
		expect(resultOf(recorded).endsAt).toBe('2026-09-02T09:00:00+09:00');

		const listed = await asSample('task_list', { opportunityHint: 'ABC상사 도입', scope: 'all' });
		expect((resultOf(listed).tasks as { content: string }[]).map((task) => task.content)).toContain(
			'킥오프 미팅'
		);
	});

	test('go in the calendar when the requester asks for it', async () => {
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			opportunityHint: 'ABC상사 도입',
			title: '2차 미팅',
			kind: 'meeting',
			business: '영업',
			isEvent: true,
			startsAt: '2026-09-10T10:00:00+09:00',
			endsAt: '2026-09-10T11:00:00+09:00',
			location: '회의실',
			notifyMinutesBefore: 30
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).isEvent).toBe(true);
		expect(resultOf(recorded).location).toBe('회의실');
		expect(resultOf(recorded).notifyMinutesBefore).toBe(30);
		expect(resultOf(recorded).business).toBe('영업');
		expect(resultOf(recorded).opportunityID).toBe(
			opportunitiesOf(await asSample('crm_opportunity_list', {}))[0].opportunityID
		);
	});

	test('are listed for the board with the colours the company gave its labels', async () => {
		const listed = await asSample('crm_activity_list', { opportunityHint: 'ABC상사 도입' });

		const titles = activitiesOf(listed).map((activity) => activity.title);
		expect(titles).toContain('킥오프 미팅');
		expect(titles).toContain('2차 미팅');
		expect(activitiesOf(listed).map((activity) => activity.kind)).toContain('stage_change');
		expect((resultOf(listed).registeredLabels as { businesses: unknown[] }).businesses).toEqual([
			{ name: '영업', color: '#2563eb' }
		]);
	});

	test('are changed in place, keeping the deal they belong to', async () => {
		const changed = await asSample('crm_activity_save', {
			activityHint: '킥오프 미팅',
			note: '예산까지 들었다',
			status: 'completed'
		});

		expect(changed.status).toBe(200);
		expect(resultOf(changed).content).toBe('예산까지 들었다');
		expect(resultOf(changed).taskStatus).toBe('completed');
		expect(resultOf(changed).startsAt).toBe('2026-09-02T09:00:00+09:00');
		expect(resultOf(changed).opportunityID).not.toBe('');
	});

	test('the automatic stage-change entry is edited without its system kind needing a registered label', async () => {
		const listed = await asAdmin('crm_activity_list', { opportunityHint: 'ABC상사 도입' });
		const stageChange = activitiesOf(listed).find((activity) => activity.kind === 'stage_change') as {
			activityID: string;
		};
		expect(stageChange).toBeDefined();

		const edited = await asAdmin('crm_activity_save', {
			activityHint: stageChange.activityID,
			kind: 'stage_change',
			note: '단계 이력에 메모를 남겼다'
		});

		expect(edited.status).toBe(200);
		expect(resultOf(edited).content).toBe('단계 이력에 메모를 남겼다');
		expect(resultOf(edited).kind).toBe('stage_change');
	});

	test('name the organization they are with, and leave stage changes to the record', async () => {
		const unnamed = await asSample('crm_activity_save', { title: '어디의 일인지 모르는 활동' });
		expect(unnamed.status).toBe(400);
		expect(errorOf(unnamed)).toContain('organization');

		const invented = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '직접 쓴 단계 변경',
			kind: 'stage_change'
		});
		expect(invented.status).toBe(400);
		expect(errorOf(invented)).toContain('moving the deal');
	});

	test('start and end today, all day, when they name no dates', async () => {
		const today = dayIn('Asia/Seoul', now);
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '날짜 없이 남긴 메모',
			kind: 'meeting'
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).isWholeDay).toBe(true);
		expect(resultOf(recorded).startsAt).toBe(today);
		expect(resultOf(recorded).endsAt).toBe(today);
	});

	test('completed with last week\'s date show on last week\'s board and not on this one', async () => {
		const lastMonday = weekWindow('Asia/Seoul', now, -1, -1).from;
		const thisMonday = dayShifted(lastMonday, 7);
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '지난주에 끝낸 미팅',
			kind: 'meeting',
			status: 'completed',
			startsAt: dayShifted(lastMonday, 1),
			endsAt: dayShifted(lastMonday, 1)
		});
		expect(recorded.status).toBe(200);
		const activityID = String(resultOf(recorded).activityID);

		const lastWeek = await asSample('task_board_get', { boardWeek: lastMonday, scope: 'all' });
		const thisWeek = await asSample('task_board_get', { boardWeek: thisMonday, scope: 'all' });

		const onLastWeek = (resultOf(lastWeek).tasks as { taskID: string; endDate: string }[]).find((task) => task.taskID === activityID);
		expect(onLastWeek?.endDate).toBe(dayShifted(lastMonday, 1));
		expect((resultOf(thisWeek).tasks as { taskID: string }[]).map((task) => task.taskID)).not.toContain(activityID);
	});

	test('on the calendar stay on the board as the same row, which the board can edit', async () => {
		const today = dayIn('Asia/Seoul', now);
		const thisMonday = weekWindow('Asia/Seoul', now, 0, 0).from;
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '캘린더에 올린 방문',
			kind: 'meeting',
			isEvent: true,
			startsAt: today,
			endsAt: today
		});
		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).isEvent).toBe(true);
		const activityID = String(resultOf(recorded).activityID);
		const ordinary = await asSample('event_add', { title: 'CRM 아닌 일정', startsAt: today, endsAt: today, isWholeDay: true });
		expect(ordinary.status).toBe(200);

		const board = await asSample('task_board_get', { boardWeek: thisMonday, scope: 'all' });
		const boardIDs = (resultOf(board).tasks as { taskID: string }[]).map((task) => task.taskID);
		expect(boardIDs).toContain(activityID);
		expect(boardIDs).not.toContain(String(resultOf(ordinary).eventID));
		const card = (resultOf(board).tasks as { taskID: string; startDate: string; endDate: string }[]).find((task) => task.taskID === activityID);
		expect([card?.startDate, card?.endDate]).toEqual([today, today]);

		const calendar = await asSample('event_list', { weekFrom: 0, scope: 'all' });
		expect((resultOf(calendar).events as { eventID: string }[]).map((event) => event.eventID)).toContain(activityID);

		const edited = await asSample('task_update', { taskHint: activityID, note: '보드에서 고쳤다' });
		expect(edited.status).toBe(200);
		const listed = activitiesOf(await asSample('crm_activity_list', { organizationHint: 'ABC상사' }));
		const held = listed.find((activity) => activity.activityID === activityID);
		expect(held?.content).toBe('보드에서 고쳤다');
		expect(held?.isEvent).toBe(true);
	});

	test('created on the calendar as completed end today, the way a task does', async () => {
		const today = dayIn('Asia/Seoul', now);
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '미리 끝낸 방문',
			kind: 'meeting',
			status: 'completed',
			isEvent: true,
			startsAt: dayShifted(today, -1),
			endsAt: dayShifted(today, 3)
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).isEvent).toBe(true);
		expect(resultOf(recorded).endsAt).toBe(today);
	});

	test('keep the size the board gave them when the calendar edits them', async () => {
		const today = dayIn('Asia/Seoul', now);
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '캘린더에서 고친 방문',
			kind: 'meeting',
			isEvent: true,
			startsAt: today,
			endsAt: today
		});
		const activityID = String(resultOf(recorded).activityID);
		expect((await asSample('task_update', { taskHint: activityID, size: 'L' })).status).toBe(200);

		const moved = await asSample('event_update', { eventHint: activityID, title: '캘린더에서 고친 방문 (변경)' });
		expect(moved.status).toBe(200);

		const held = activitiesOf(await asSample('crm_activity_list', { organizationHint: 'ABC상사' }))
			.find((activity) => activity.activityID === activityID);
		expect(held?.title).toBe('캘린더에서 고친 방문 (변경)');
		expect(held?.size).toBe('L');
	});

	test('go on and off the calendar as the one row they are', async () => {
		const recorded = await asSample('crm_activity_save', {
			organizationHint: 'ABC상사',
			title: '나중에 캘린더에 올린 통화',
			kind: 'meeting'
		});
		const activityID = String(resultOf(recorded).activityID);

		const ticked = await asSample('crm_activity_save', { activityHint: activityID, isEvent: true });
		expect(ticked.status).toBe(200);
		expect(resultOf(ticked).activityID).toBe(activityID);
		expect(resultOf(ticked).isEvent).toBe(true);

		const unticked = await asSample('crm_activity_save', { activityHint: activityID, isEvent: false });
		expect(unticked.status).toBe(200);
		expect(resultOf(unticked).activityID).toBe(activityID);
		expect(resultOf(unticked).isEvent).toBe(false);
	});
});

describe('what a company puts away', () => {
	test('leaves the CRM screens while everything recorded against it stays', async () => {
		const deal = await asSample('crm_opportunity_archive', { opportunityHint: 'ABC상사 도입' });
		expect(deal.status).toBe(200);
		expect(resultOf(deal).name).toBe('ABC상사 도입');

		const contact = await asSample('crm_contact_archive', { contactHint: 'yesi' });
		expect(contact.status).toBe(200);

		expect(opportunitiesOf(await asSample('crm_opportunity_list', {}))).toHaveLength(0);
		expect(resultOf(await asSample('crm_contact_list', {})).contacts as unknown[]).toHaveLength(0);
		expect(
			opportunitiesOf(await asSample('crm_opportunity_list', { includeArchived: true }))
		).toHaveLength(1);
		expect(activitiesOf(await asSample('crm_activity_list', {})).length).toBeGreaterThan(0);
	});
});

describe('records imported from a file', () => {
	const importedName = '가져온상사';
	const heldName = '가져오기기존상사';

	beforeAll(async () => {
		await asSample('crm_organization_add', { name: heldName, address: '대구' });
	}, networkHookTimeout);

	function importedRowsOf(answer: { body: unknown }): Record<string, unknown>[] {
		return resultOf(answer).rows as Record<string, unknown>[];
	}

	async function organizationNamed(name: string): Promise<Record<string, unknown> | undefined> {
		return organizationsOf(await asSample('crm_organization_list', { query: name })).find(
			(organization) => organization.name === name
		);
	}

	test('are checked without a write on a dry run', async () => {
		const checked = await asSample('crm_record_import', {
			kind: 'organization',
			dryRun: true,
			rows: [
				{ line: 2, name: importedName, types: ['고객'] },
				{ line: 3, name: ` ${heldName} ` },
				{ line: 4, name: '' },
				{ line: 5, name: '유형없는상사', types: ['없는유형'] },
				{ line: 6, name: importedName }
			]
		});

		expect(checked.status).toBe(200);
		expect(resultOf(checked)).toMatchObject({ dryRun: true, written: false, created: 1, skipped: 1, refused: 3 });
		expect(importedRowsOf(checked).map((row) => [row.line, row.outcome, row.problems])).toEqual([
			[2, 'create', []],
			[3, 'skip', []],
			[4, 'refused', ['name_missing']],
			[5, 'refused', ['type_unknown']],
			[6, 'refused', ['name_repeated_in_file']]
		]);
		expect(importedRowsOf(checked)[1].duplicateOfID).not.toBe('');
		expect(await organizationNamed(importedName)).toBeUndefined();
	});

	test('write nothing when one record is refused', async () => {
		const refused = await asSample('crm_record_import', {
			kind: 'organization',
			rows: [
				{ line: 2, name: importedName },
				{ line: 3, name: '담당없는상사', ownerPersonHint: '없는사람' }
			]
		});

		expect(refused.status).toBe(200);
		expect(resultOf(refused)).toMatchObject({ written: false, refused: 1 });
		expect(importedRowsOf(refused)[1].problems).toEqual(['owner_unresolved']);
		expect(await organizationNamed(importedName)).toBeUndefined();
	});

	test('add new organizations and update a held one without blanking what the file leaves empty', async () => {
		const held = await organizationNamed(heldName);
		const imported = await asSample('crm_record_import', {
			kind: 'organization',
			rows: [
				{ line: 2, name: importedName, types: ['customer'], address: '부산, "해운대"\n2층', ownerPersonHint: '이샘플' },
				{ line: 3, name: heldName, onDuplicate: 'update', description: '가져오기로 고친 설명', address: '' }
			]
		});

		expect(resultOf(imported)).toMatchObject({ written: true, created: 1, updated: 1, refused: 0 });
		const added = await organizationNamed(importedName);
		expect(added).toMatchObject({ address: '부산, "해운대"\n2층', types: ['customer'], ownerPersonID: sampleID, status: 'active' });
		expect(added?.organizationID).toBe(importedRowsOf(imported)[0].recordID);
		expect(await organizationNamed(heldName)).toMatchObject({
			organizationID: held?.organizationID,
			address: held?.address,
			description: '가져오기로 고친 설명'
		});
	});

	test('add contacts to the organization their file names', async () => {
		const imported = await asSample('crm_record_import', {
			kind: 'contact',
			dryRun: true,
			rows: [
				{ line: 2, name: '박예시', organizationName: importedName, email: 'yesi@example.com' },
				{ line: 3, name: '연락없음', organizationName: importedName },
				{ line: 4, name: '주소틀림', organizationName: importedName, email: 'not-an-email' },
				{ line: 5, name: '소속없음', organizationName: '없는상사', phoneNumber: '010-0000-0000' },
				{ line: 6, name: '섞인칸', organizationName: importedName, email: 'mixed@example.com', address: '서울' }
			]
		});

		expect(importedRowsOf(imported).map((row) => row.problems)).toEqual([
			[],
			['contact_unreachable'],
			['email_invalid'],
			['organization_unknown'],
			['field_of_other_kind']
		]);

		const written = await asSample('crm_record_import', {
			kind: 'contact',
			rows: [{ line: 2, name: '박예시', organizationName: importedName, email: 'yesi@example.com' }]
		});
		expect(resultOf(written)).toMatchObject({ written: true, created: 1 });
		const organization = await organizationNamed(importedName);
		const contacts = resultOf(
			await asSample('crm_contact_list', { organizationHint: String(organization?.organizationID) })
		).contacts as Record<string, unknown>[];
		expect(contacts.map((contact) => contact.name)).toEqual(['박예시']);
	});

	test('stay inside the importing company', async () => {
		const imported = await asOutsider('crm_record_import', {
			kind: 'organization',
			rows: [{ line: 2, name: importedName, onDuplicate: 'update', description: '다른 회사' }]
		});

		expect(resultOf(imported)).toMatchObject({ written: true, created: 1, updated: 0 });
		expect((await organizationNamed(importedName))?.description).not.toBe('다른 회사');
	});
});

describe('records imported from a workspace file the call carries', () => {
	const filePath = '/workspace/shared/inbox/clients.csv';
	const fileName = '파일로가져온상사';
	const heldName = '파일가져오기기존상사';

	beforeAll(async () => {
		await asSample('crm_organization_add', { name: heldName, address: '광주', description: '그대로 둘 설명' });
	}, networkHookTimeout);

	function carrying(content: string, path = filePath) {
		return [{ workspacePath: path, filename: 'clients.csv', contentBase64: Buffer.from(content).toString('base64') }];
	}

	async function importedFrom(content: string, input: Record<string, unknown>) {
		return heldToTheContract(
			'crm_record_import',
			await runToolOverTheRecord(sample, client, sampleID, 'crm_record_import', { kind: 'organization', attachments: [filePath], ...input }, now, leavesTaskLabelsUndecided, carrying(content))
		);
	}

	async function previewedFrom(content: string, input: Record<string, unknown>) {
		return previewToolOverTheRecord(sample, client, sampleID, 'crm_record_import', { kind: 'organization', attachments: [filePath], ...input }, now, carrying(content));
	}

	async function organizationNamed(name: string): Promise<Record<string, unknown> | undefined> {
		return organizationsOf(await asSample('crm_organization_list', { query: name })).find(
			(organization) => organization.name === name
		);
	}

	const columns = { name: '회사명', status: '상태', address: '주소' };

	test('show what the import would do before anyone approves it', async () => {
		const previewed = await previewedFrom(`회사명,상태,주소\n${fileName},활성,"부산, ""해운대"""\n${heldName},,\n`, {
			columns,
			onDuplicate: 'update'
		});

		expect(previewed.status).toBe(200);
		expect((previewed.body as { target: unknown }).target).toEqual({
			inputField: 'attachments',
			ids: [filePath],
			title: 'clients.csv',
			preview: 'creates 1, updates 1, skips 0 already held'
		});
		expect(await organizationNamed(fileName)).toBeUndefined();
	});

	test('refuse before the question a file with a line that cannot be imported, naming the line', async () => {
		const previewed = await previewedFrom(`회사명,상태\n${fileName},활성\n,활성\n`, { columns: { name: '회사명', status: '상태' } });

		expect(previewed.status).toBe(400);
		expect(errorOf(previewed)).toContain('line 3: name_missing');
	});

	test('write nothing when one line of the file is refused', async () => {
		const imported = await importedFrom(`회사명,상태\n${fileName},활성\n유령상사,없는상태\n`, { columns: { name: '회사명', status: '상태' } });

		expect(resultOf(imported)).toMatchObject({ written: false, refused: 1 });
		expect(await organizationNamed(fileName)).toBeUndefined();
	});

	test('write every line, reading a status by its label and keeping what the file leaves empty', async () => {
		const held = await organizationNamed(heldName);
		const imported = await importedFrom(`회사명,상태,주소\n${fileName},활성,"부산, ""해운대"""\n${heldName},,\n`, {
			columns,
			onDuplicate: 'update'
		});

		expect(resultOf(imported)).toMatchObject({ written: true, created: 1, updated: 1, refused: 0 });
		expect(await organizationNamed(fileName)).toMatchObject({ status: 'active', address: '부산, "해운대"' });
		expect(await organizationNamed(heldName)).toMatchObject({ address: held?.address, description: '그대로 둘 설명' });
	});

	test('refuse a column the file does not have, naming the ones it does', async () => {
		const imported = await importedFrom('회사명\n샘플상사\n', { columns: { name: '상호' } });

		expect(imported.status).toBe(400);
		expect(errorOf(imported)).toContain('"상호"');
		expect(errorOf(imported)).toContain('"회사명"');
	});

	test('refuse a contact file with no column for its organization', async () => {
		const imported = await importedFrom('이름\n박예시\n', { kind: 'contact', columns: { name: '이름' } });

		expect(imported.status).toBe(400);
		expect(errorOf(imported)).toContain('organizationName');
	});

	test('refuse a file over 5 MB before reading it', async () => {
		const imported = await importedFrom(`회사명\n${'가'.repeat(2 * 1024 * 1024)}\n`, { columns: { name: '회사명' } });

		expect(imported.status).toBe(400);
		expect(errorOf(imported)).toContain('at most 5 MB');
	});

	test('refuse a file the call did not carry', async () => {
		const imported = heldToTheContract(
			'crm_record_import',
			await runToolOverTheRecord(sample, client, sampleID, 'crm_record_import', { kind: 'organization', attachments: [filePath], columns: { name: '회사명' } }, now, leavesTaskLabelsUndecided)
		);

		expect(imported.status).toBe(400);
		expect(errorOf(imported)).toContain('carried none');
	});

	test('ask without a preview when the file did not come with the question', async () => {
		const previewed = await previewToolOverTheRecord(sample, client, sampleID, 'crm_record_import', { kind: 'organization', attachments: [filePath], columns: { name: '회사명' } }, now);

		expect(previewed).toEqual({ status: 200, body: { tool: 'crm_record_import', target: null } });
	});

	test('refuse a call that names a file and sends rows', async () => {
		const imported = await importedFrom('회사명\n샘플상사\n', { columns: { name: '회사명' }, rows: [{ line: 2, name: '샘플상사' }] });

		expect(imported.status).toBe(400);
		expect(errorOf(imported)).toContain('not both');
	});
});
