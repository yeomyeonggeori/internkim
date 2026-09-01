import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { whoAnswersFor } = await import('../../src/lib/server/who-answers');

const networkHookTimeout = 60_000;
const record = controlPlane({ projectURL, serviceRoleKey });
const slug = `who-answers-${Date.now()}`;

let companyID = '';
let adminID = '';
let representativeID = '';
let sampleID = '';

async function seatRepresentative(memberID: string): Promise<void> {
	const { data, error } = await record
		.from('circle')
		.insert({ company_id: companyID, name: 'representative' })
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
	const seated = await record.from('circle_member').insert({ circle_id: data.id, member_id: memberID });
	if (seated.error) throw new Error(seated.error.message);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		record,
		{ name: 'Who Answers', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	await record.from('member').update({ is_admin: true, status: 'active' }).eq('id', adminID);

	representativeID = await addMember(record, companyID, `${slug}-representative@example.test`);
	sampleID = await addMember(record, companyID, `${slug}-sample@example.test`);
	await record.from('member').update({ is_admin: true, status: 'active' }).eq('id', representativeID);
	await record.from('member').update({ status: 'active' }).eq('id', sampleID);
}, networkHookTimeout);

afterAll(async () => {
	if (companyID) await record.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('who is asked to act on somebody else behalf', () => {
	test('is every administrator while the company names no representative', async () => {
		const asked = await whoAnswersFor(record, companyID, sampleID);

		expect(asked.sort()).toEqual([adminID, representativeID].sort());
	});

	test('is the representative alone once the company names one', async () => {
		await seatRepresentative(representativeID);

		expect(await whoAnswersFor(record, companyID, sampleID)).toEqual([representativeID]);
	});

	test('never asks the person who asked', async () => {
		expect(await whoAnswersFor(record, companyID, representativeID)).toEqual([]);
	});
});
