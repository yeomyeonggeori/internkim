import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord, previewToolOverTheRecord } = await import('../../src/lib/server/public-api/record');
const { buildCapabilityToolCatalog } = await import('../../src/lib/server/public-api/catalog/tools');
const { protocolVersion } = await import('../../src/lib/server/public-api/catalog/protocol');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `company-settings-${Date.now()}`;
const now = new Date();

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Company Settings Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);

	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function asSample(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(sample, client, sampleID, name, input, now);
}

function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(admin, client, adminID, name, input, now);
}

function resultOf(answer: { body: unknown }): Record<string, unknown> {
	return (answer.body as { result: Record<string, unknown> }).result;
}

function errorOf(answer: { body: unknown }): string {
	return (answer.body as { error?: string }).error ?? '';
}

function descriptorOf(name: string) {
	const catalog = buildCapabilityToolCatalog(protocolVersion);
	const descriptor = catalog.tools.find((tool) => tool.name === name);
	if (!descriptor) throw new Error(`${name} is not in the catalog`);
	return descriptor;
}

const workPolicy = {
	workMode: 'fixed',
	workingWeekdays: [1, 2, 3, 4, 5],
	dailyTargetMinutes: 480,
	weeklyTargetMinutes: 2400,
	referenceStartTime: '09:00',
	fixedStartTime: '09:00',
	fixedEndTime: '18:00',
	coreTimeEnabled: false,
	coreStartTime: '',
	coreEndTime: '',
	breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
	nightStartTime: '22:00',
	nightEndTime: '06:00'
};

describe('what the company is set to', () => {
	test('is answered to anybody who works here', async () => {
		const answered = await asSample('company_settings_get');

		expect(answered.status).toBe(200);
		expect(resultOf(answered).timeZone).toBe('Asia/Seoul');
		expect(resultOf(answered).country).toBe('KR');
		expect(resultOf(answered).currencyCode).toBe('KRW');
		expect(resultOf(answered).profileImageURL).toBeNull();
	});

	test('is an administrator to change, and the record says so to anybody else', async () => {
		const refused = await asSample('company_settings_update', { currencyCode: 'USD' });

		expect(refused.status).toBe(403);
		expect(errorOf(refused)).toContain('administrator');
		expect(resultOf(await asAdmin('company_settings_get')).currencyCode).toBe('KRW');
	});

	test('changes only what the call names, and answers what it is set to now', async () => {
		const written = await asAdmin('company_settings_update', {
			currencyCode: 'usd',
			workLocations: [{ name: '사무실' }, { name: '재택', color: '#112233' }]
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).currencyCode).toBe('USD');
		expect(resultOf(written).timeZone).toBe('Asia/Seoul');
		expect(resultOf(written).workLocations).toEqual([
			{ name: '사무실', color: null },
			{ name: '재택', color: '#112233' }
		]);
	});

	test('says whether everybody may see the whole company, and lets it be turned off', async () => {
		expect(resultOf(await asSample('company_settings_get')).teamViewVisibleToAll).toBe(true);

		const hidden = await asAdmin('company_settings_update', { teamViewVisibleToAll: false });
		expect(hidden.status).toBe(200);
		expect(resultOf(hidden).teamViewVisibleToAll).toBe(false);
		expect(resultOf(await asSample('company_settings_get')).teamViewVisibleToAll).toBe(false);
		expect(resultOf(await asAdmin('company_settings_get')).currencyCode).toBe('USD');

		const shown = await asAdmin('company_settings_update', { teamViewVisibleToAll: true });
		expect(resultOf(shown).teamViewVisibleToAll).toBe(true);
	});

	test('refuses a change that names nothing, and a time zone that is not one', async () => {
		expect((await asAdmin('company_settings_update', {})).status).toBe(400);
		expect((await asAdmin('company_settings_update', { timeZone: 'Mars/Olympus' })).status).toBe(422);
	});

	test('is a wide enough change that the descriptor asks before it is made', () => {
		expect(descriptorOf('company_settings_update').requiresApproval).toBe(true);
		expect(descriptorOf('attendance_work_policy_set').requiresApproval).toBe(true);
		expect(descriptorOf('attendance_leave_policy_set').requiresApproval).toBe(true);
		expect(descriptorOf('company_holiday_delete').requiresApproval).toBe(true);
		expect(descriptorOf('company_settings_get').requiresApproval).toBeUndefined();
		expect(descriptorOf('company_holiday_add').requiresApproval).toBeUndefined();
	});
});

describe('the company master profile', () => {
	test('starts empty and says which core fields it is missing', async () => {
		const answered = await asAdmin('company_info_get', { language: 'ko' });

		expect(answered.status).toBe(200);
		expect(resultOf(answered).name).toBe('');
		expect(resultOf(answered).missingFields).toEqual([
			'name',
			'representative',
			'address',
			'bankAccount',
			'phone',
			'email'
		]);
		expect(resultOf(answered).representativeTitle).toBe('대표이사');
	});

	test('is written a language slot at a time and read back for that language', async () => {
		const written = await asAdmin('company_info_set', {
			language: 'ko',
			name: '주식회사 예시',
			representative: '이샘플',
			address: '서울특별시 예시구 예시로 1',
			bankAccount: '예시은행 123-456-789 주식회사 예시',
			phone: '02-000-0000',
			email: 'hello@example.com',
			legalAttributes: '{"사업자등록번호": "123-45-67890"}'
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).missingFields).toEqual([]);
		expect(resultOf(written).legalAttributes).toEqual([
			{ label: '사업자등록번호', value: '123-45-67890' }
		]);

		const english = await asAdmin('company_info_get', { language: 'en' });
		expect(resultOf(english).name).toBe('주식회사 예시');
		expect(resultOf(english).missingFields).toContain('name');
		expect(resultOf(english).representativeTitle).toBe('CEO');
	});

	test('keeps the fields a later call does not name', async () => {
		await asAdmin('company_info_set', { language: 'ko', website: 'https://example.com' });
		const answered = await asAdmin('company_info_get', { language: 'ko' });

		expect(resultOf(answered).website).toBe('https://example.com');
		expect(resultOf(answered).name).toBe('주식회사 예시');
		expect(resultOf(answered).updatedAt).not.toBe('');
	});

	test('is an administrator to write, and everyone here may read it', async () => {
		const refused = await asSample('company_info_set', { language: 'ko', name: '내가 쓴 이름' });

		expect(refused.status).toBe(403);
		expect(resultOf(await asSample('company_info_get', { language: 'ko' })).name).toBe('주식회사 예시');
	});

	test('refuses legal attributes that are not a label-to-value object', async () => {
		const refused = await asAdmin('company_info_set', { language: 'ko', legalAttributes: '["없음"]' });

		expect(refused.status).toBe(400);
	});
});

describe('the days the company is closed', () => {
	test('are added by name and date, and listed earliest first', async () => {
		const added = await asAdmin('company_holiday_add', {
			date: '2026-10-03',
			name: '개천절',
			recursAnnually: true
		});
		await asAdmin('company_holiday_add', { date: '2026-01-01', name: '신정' });

		expect(added.status).toBe(200);
		expect(resultOf(added).recursAnnually).toBe(true);

		const listed = await asSample('company_holiday_list');
		const holidays = resultOf(listed).holidays as { name: string; date: string }[];
		expect(holidays.map((holiday) => holiday.date)).toEqual(['2026-01-01', '2026-10-03']);
	});

	test('are refused a second holiday on a day one already has', async () => {
		const refused = await asAdmin('company_holiday_add', { date: '2026-10-03', name: '또 개천절' });

		expect(refused.status).toBe(409);
		expect(errorOf(refused)).toContain('개천절');
	});

	test('are counted into the year that was asked for', async () => {
		const listed = await asSample('company_holiday_list', { year: 2027 });

		const holidays = resultOf(listed).holidays as { name: string }[];
		expect(holidays.map((holiday) => holiday.name)).toEqual(['개천절']);
	});

	test('resolve a hint by id, by date, and by a name only one of them carries', async () => {
		const listed = await asAdmin('company_holiday_list');
		const first = (resultOf(listed).holidays as { holidayID: string }[])[0];

		expect(resultOf(await asAdmin('company_holiday_update', { holidayHint: first.holidayID, name: '신정 연휴' })).name).toBe('신정 연휴');
		expect(resultOf(await asAdmin('company_holiday_update', { holidayHint: '2026-10-03', name: '개천절' })).date).toBe('2026-10-03');
		expect(resultOf(await asAdmin('company_holiday_update', { holidayHint: '개천절', recursAnnually: false })).recursAnnually).toBe(false);
	});

	// A name only one holiday carries resolves even when another holiday's name
	// starts with it, because an exact name is the second rung of the ladder. A
	// fragment several carry and none is, is the rung that asks.
	test('ask which one when a fragment matches more than one, and resolve nothing', async () => {
		await asAdmin('company_holiday_add', { date: '2026-05-05', name: '개천절 대체' });

		const refused = await asAdmin('company_holiday_update', { holidayHint: '개천', name: '무엇' });

		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode: string }).errorCode).toBe('interaction_required');
		expect((refused.body as { candidates: unknown[] }).candidates.length).toBe(2);
		expect(resultOf(await asAdmin('company_holiday_list')).count).toBe(3);
	});

	test('say so when nothing is near what was asked for', async () => {
		const refused = await asAdmin('company_holiday_delete', { holidayHint: '아무 날도 아닌 날' });

		expect(refused.status).toBe(409);
		expect((refused.body as { errorCode: string }).errorCode).toBe('company_holiday_not_found');
	});

	test('are looked at before they are removed, through the same hint the call uses', async () => {
		const looked = await previewToolOverTheRecord(admin, client, adminID, 'company_holiday_delete', { holidayHint: '2026-05-05' }, now);

		expect(looked.status).toBe(200);
		expect((looked.body as { target: { title: string } }).target.title).toContain('개천절 대체');
	});

	test('are removed by an administrator, and the list stops carrying them', async () => {
		const removed = await asAdmin('company_holiday_delete', { holidayHint: '2026-05-05' });
		expect(removed.status).toBe(200);
		expect(resultOf(removed).date).toBe('2026-05-05');

		const listed = await asAdmin('company_holiday_list');
		const holidays = resultOf(listed).holidays as { date: string }[];
		expect(holidays.map((holiday) => holiday.date)).toEqual(['2026-01-01', '2026-10-03']);
	});

	test('are not a colleague to add or remove', async () => {
		expect((await asSample('company_holiday_add', { date: '2026-12-25', name: '성탄절' })).status).toBe(403);
		expect((await asSample('company_holiday_delete', { holidayHint: '2026-10-03' })).status).toBe(403);
	});

	// The holidays and the team view rule share one column, so a write of either
	// that does not merge takes the other with it.
	test('outlive a change to whether the team view is visible to everybody', async () => {
		await asAdmin('company_settings_update', { teamViewVisibleToAll: false });

		const listed = await asAdmin('company_holiday_list');
		const holidays = resultOf(listed).holidays as { date: string }[];
		expect(holidays.map((holiday) => holiday.date)).toEqual(['2026-01-01', '2026-10-03']);

		await asAdmin('company_settings_update', { teamViewVisibleToAll: true });
	});
});

describe('the hours the company works', () => {
	test('are the default until somebody saves one', async () => {
		const answered = await asSample('attendance_work_policy_get');

		expect(answered.status).toBe(200);
		expect(resultOf(answered).timeZone).toBe('Asia/Seoul');
		expect(resultOf(answered).policy).toBeNull();
		expect((resultOf(answered).people as unknown[]).length).toBe(2);
	});

	test('are an administrator to set, and cover all of time on the first save', async () => {
		const refused = await asSample('attendance_work_policy_set', workPolicy);
		expect(refused.status).toBe(403);

		const written = await asAdmin('attendance_work_policy_set', workPolicy);
		expect(written.status).toBe(200);
		expect(resultOf(written).effectiveDate).toBe('1970-01-01');

		const read = await asAdmin('attendance_work_policy_get');
		const policy = resultOf(read).policy as { revisions: { workMode: string }[] };
		expect(policy.revisions).toHaveLength(1);
		expect(policy.revisions[0].workMode).toBe('fixed');
	});

	test('keep the policy that was in force when a later one is saved', async () => {
		const written = await asAdmin('attendance_work_policy_set', {
			...workPolicy,
			workMode: 'flexible',
			fixedStartTime: '',
			fixedEndTime: '',
			coreTimeEnabled: true,
			coreStartTime: '11:00',
			coreEndTime: '16:00'
		});

		expect(written.status).toBe(200);
		expect(written.body).not.toHaveProperty('effectiveDate', '1970-01-01');
		const policy = resultOf(written).policy as { revisions: { workMode: string }[] };
		expect(policy.revisions.map((revision) => revision.workMode)).toEqual(['fixed', 'flexible']);
	});

	test('refuse a policy whose fixed hours do not add up to the day it claims', async () => {
		const refused = await asAdmin('attendance_work_policy_set', {
			...workPolicy,
			dailyTargetMinutes: 300,
			weeklyTargetMinutes: 1500
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toBe('workPolicy fixed hours do not match dailyTargetMinutes');
	});

	test('refuse a week whose minutes are not the days it is made of', async () => {
		const refused = await asAdmin('attendance_work_policy_set', {
			...workPolicy,
			weeklyTargetMinutes: 1500
		});

		expect(refused.status).toBe(400);
		expect(errorOf(refused)).toBe('workPolicy weeklyTargetMinutes is invalid');
	});
});

describe('the leave the company offers', () => {
	test('is the default policy, with the annual grant the company row carries', async () => {
		const answered = await asSample('attendance_leave_policy_get');

		expect(answered.status).toBe(200);
		expect(resultOf(answered).balanceTrackingMode).toBe('unlimited');
		expect((resultOf(answered).leaveTypes as { id: string }[]).map((leaveType) => leaveType.id)).toContain('annual');
	});

	test('is an administrator to set, and the annual grant lands on the company row', async () => {
		const answered = await asAdmin('attendance_leave_policy_get');
		const leaveTypes = resultOf(answered).leaveTypes as Record<string, unknown>[];
		const written = await asAdmin('attendance_leave_policy_set', {
			balanceTrackingMode: 'managed',
			fiscalYearStartMonth: 3,
			fiscalYearStartDay: 1,
			leaveTypes: leaveTypes.map((leaveType) =>
				leaveType.id === 'annual' ? { ...leaveType, grantAmountMilliDays: 18000 } : leaveType
			)
		});

		expect(written.status).toBe(200);
		expect(resultOf(written).balanceTrackingMode).toBe('managed');
		expect(resultOf(written).fiscalYearStartMonth).toBe(3);
		expect(resultOf(await asAdmin('company_settings_get')).leaveDays).toBe(18);
	});

	test('refuses a fiscal year that starts on a day that is not one', async () => {
		const answered = await asAdmin('attendance_leave_policy_get');
		const refused = await asAdmin('attendance_leave_policy_set', {
			balanceTrackingMode: 'managed',
			fiscalYearStartMonth: 2,
			fiscalYearStartDay: 29,
			leaveTypes: resultOf(answered).leaveTypes
		});

		expect(refused.status).toBe(422);
		expect(errorOf(refused)).toContain('real calendar date');
	});

	test('is not a colleague to set', async () => {
		const answered = await asAdmin('attendance_leave_policy_get');
		const refused = await asSample('attendance_leave_policy_set', {
			balanceTrackingMode: 'unlimited',
			fiscalYearStartMonth: 1,
			fiscalYearStartDay: 1,
			leaveTypes: resultOf(answered).leaveTypes
		});

		expect(refused.status).toBe(403);
	});
});
