import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';
import { heldToTheContract } from './tool-answers';
import { leavesTaskLabelsUndecided } from '../../src/lib/server/public-api/record/task-labels';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { runToolOverTheRecord, previewToolOverTheRecord } = await import('../../src/lib/server/public-api/record');
const { buildCapabilityToolCatalog } = await import('../../src/lib/server/public-api/catalog/tools');
const { protocolVersion } = await import('../../src/lib/server/public-api/catalog/protocol');
const { newestServiceFile } = await import('../../src/lib/server/public-api/record/service-files');
const { assetBucket } = await import('../../src/lib/server/public-api/asset-address');
const { dayIn } = await import('../../src/lib/server/public-api/record/days');

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
	const session = await sessionForMember({ projectURL, serviceRoleKey, signingKey }, memberID);
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

async function asSample(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(sample, client, sampleID, name, input, now, leavesTaskLabelsUndecided));
}

async function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return heldToTheContract(name, await runToolOverTheRecord(admin, client, adminID, name, input, now, leavesTaskLabelsUndecided));
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

const aPixelPNG = Uint8Array.from(
	atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=='),
	(character) => character.charCodeAt(0)
);

const uploadedPaths: string[] = [];

type KeptDocument = { id: string; category_code: string; document_date: string; supersedes: string | null; storage_path: string | null };

async function documentAt(storagePath: string): Promise<KeptDocument> {
	const { data } = await client
		.from('company_document')
		.select('id, category_code, document_date, supersedes, storage_path')
		.eq('storage_path', storagePath)
		.single<KeptDocument>();
	if (!data) throw new Error(`no document keeps ${storagePath}`);
	return data;
}

async function anUploadedImage(image: 'seal' | 'logo', fileName: string, date?: string): Promise<string> {
	const asked = await asAdmin('company_image_upload', { image, fileName, ...(date ? { date } : {}) });
	expect(asked.status).toBe(200);
	const put = await fetch(String(resultOf(asked).uploadURL), {
		method: 'PUT',
		headers: { 'content-type': 'image/png' },
		body: aPixelPNG
	});
	expect(put.ok).toBe(true);
	const storagePath = String(resultOf(asked).storagePath);
	uploadedPaths.push(storagePath);
	return storagePath;
}

async function aKeptImage(image: 'seal' | 'logo', date: string): Promise<string> {
	const storagePath = await anUploadedImage(image, `${image}.png`, date);
	expect((await asAdmin('company_info_set', { language: 'ko', [`${image}Image`]: storagePath })).status).toBe(200);
	return storagePath;
}

async function newestSealPath(): Promise<string | undefined> {
	return (await newestServiceFile(admin, companyID, 'seal'))?.storagePath;
}

const uuidPattern = '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';

describe('the company seal and logo', () => {
	afterAll(async () => {
		await client.storage.from(assetBucket).remove(uploadedPaths);
	}, networkHookTimeout);

	test('are kept as dated data room documents under a fixed name in their categories', async () => {
		const sealPath = await anUploadedImage('seal', '법인인감.png', '2026-03-01');
		const logoPath = await anUploadedImage('logo', 'logo.JPG');
		const today = dayIn('Asia/Seoul', now);

		expect(sealPath).toMatch(new RegExp(`^${companyID}/dataroom/C/CR/seal\\.2026-03-01\\.${uuidPattern}\\.png$`));
		expect(logoPath).toMatch(new RegExp(`^${companyID}/dataroom/S/SM/logo\\.${today}\\.${uuidPattern}\\.jpg$`));

		const written = await asAdmin('company_info_set', { language: 'ko', sealImage: sealPath, logoImage: logoPath });
		expect(written.status).toBe(200);
		expect(resultOf(written).name).toBe('주식회사 예시');

		expect(await documentAt(sealPath)).toMatchObject({ category_code: 'CR', document_date: '2026-03-01', supersedes: null });
		expect(await documentAt(logoPath)).toMatchObject({ category_code: 'SM', document_date: today });
		expect(await newestSealPath()).toBe(sealPath);
		const { data: company } = await client.from('company').select('profile_image').eq('id', companyID).single();
		expect(company?.profile_image).toBeNull();
	});

	test('a later seal is a new document that supersedes the one before it, which stays', async () => {
		const earlier = await newestSealPath();
		const later = await aKeptImage('seal', '2026-09-01');

		expect(await newestSealPath()).toBe(later);
		expect((await documentAt(later)).supersedes).toBe((await documentAt(String(earlier))).id);
		expect((await documentAt(String(earlier))).storage_path).toBe(String(earlier));
	});

	test('a seal dated before the newest is kept without replacing it', async () => {
		const newest = await newestSealPath();
		const backdated = await aKeptImage('seal', '2025-01-01');

		expect(await newestSealPath()).toBe(newest);
		expect((await documentAt(backdated)).supersedes).toBeNull();
	});

	test('of two seals dated the same day, the one created later is the newest', async () => {
		const sameDay = await aKeptImage('seal', '2026-09-01');

		expect(await newestSealPath()).toBe(sameDay);
	});

	test('refuse a path that company_image_upload did not answer for that image, or one nothing was put at', async () => {
		const elsewhere = await asAdmin('company_info_set', { language: 'ko', sealImage: `${companyID}/shared/company/seal.png` });
		const asked = await asAdmin('company_image_upload', { image: 'seal', fileName: 'seal.png' });
		const askedPath = String(resultOf(asked).storagePath);
		const swapped = await asAdmin('company_info_set', { language: 'ko', logoImage: askedPath });
		const neverPut = await asAdmin('company_info_set', { language: 'ko', sealImage: askedPath });

		expect(elsewhere.status).toBe(400);
		expect(swapped.status).toBe(400);
		expect(neverPut.status).toBe(409);
	});

	test('remove no kept version when given an empty path', async () => {
		const newest = await newestSealPath();
		const emptied = await asAdmin('company_info_set', { language: 'ko', sealImage: '' });

		expect(emptied.status).toBe(400);
		expect(await newestSealPath()).toBe(newest);
	});

	test('refuse an image that is no service file, not a picture, or dated on no calendar day', async () => {
		const unknownImage = await asAdmin('company_image_upload', { image: 'signature', fileName: 'sign.png' });
		const notAPicture = await asAdmin('company_image_upload', { image: 'seal', fileName: 'seal.pdf' });
		const noSuchDay = await asAdmin('company_image_upload', { image: 'seal', fileName: 'seal.png', date: '2026-02-30' });

		expect(unknownImage.status).toBe(400);
		expect(notAPicture.status).toBe(400);
		expect(noSuchDay.status).toBe(400);
	});

	test('are an administrator to keep', async () => {
		const asked = await asSample('company_image_upload', { image: 'seal', fileName: 'seal.png' });

		expect(asked.status).toBe(403);
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

		const balance = resultOf(await asAdmin('leave_balance'));
		const mine = (balance.balances as { grantedDays: number | null }[])[0];
		expect(mine.grantedDays).toBe(18);
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
