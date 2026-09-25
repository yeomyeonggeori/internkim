import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';

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
			amountMinor: 18000000,
			currencyCode: 'KRW',
			business: '영업',
			expectedCloseDate: '2026-09-30'
		});
		expect(opened.status).toBe(200);
		expect(resultOf(opened).stage).toBe('waiting');
		expect(resultOf(opened).pipeline).toBe('partnership');
		expect(resultOf(opened).expectedCloseTimeZone).toBe('Asia/Seoul');
		expect(resultOf(opened).amountMinor).toBe(18000000);
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
		expect(resultOf(changed).amountMinor).toBe(18000000);
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
		expect(resultOf(settled).baseAmountMinor).toBe(18000000);
		expect(resultOf(settled).baseCurrencyCode).toBe('KRW');
	});

	test('are reordered without moving stage, which leaves the day they last moved alone', async () => {
		const stood = opportunitiesOf(await asSample('crm_opportunity_list', {}))[0];
		const reordered = await asSample('crm_opportunity_move', { opportunityHint: 'ABC상사 도입', position: 7 });

		expect(resultOf(reordered).stagePosition).toBe(7);
		expect(resultOf(reordered).stage).toBe('done');
		expect(resultOf(reordered).stageChangedAt).toBe(stood.stageChangedAt);
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
			occurredAt: '2026-09-02T09:00:00+09:00'
		});

		expect(recorded.status).toBe(200);
		expect(resultOf(recorded).content).toBe('요구사항을 들었다');
		expect(resultOf(recorded).kind).toBe('meeting');
		expect(resultOf(recorded).isEvent).toBe(false);
		expect(new Date(resultOf(recorded).occurredAt as string).toISOString()).toBe(
			'2026-09-02T00:00:00.000Z'
		);

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
			status: 'in_progress'
		});

		expect(changed.status).toBe(200);
		expect(resultOf(changed).content).toBe('예산까지 들었다');
		expect(resultOf(changed).taskStatus).toBe('in_progress');
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
