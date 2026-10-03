import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey, signingKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey, SUPABASE_JWT_SIGNING_KEY: signingKey }
}));

const { whoAnswersFor } = await import('../../src/lib/server/who-answers');

const networkHookTimeout = 60_000;
const record = controlPlane({ projectURL, serviceRoleKey });
const slug = `who-answers-${Date.now()}`;

let companyID = '';
let adminID = '';
let secondAdminID = '';
let sampleID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		record,
		{ name: 'Who Answers', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	await record.from('member').update({ is_admin: true, status: 'active' }).eq('id', adminID);

	secondAdminID = await addMember(record, companyID, `${slug}-second-admin@example.test`);
	sampleID = await addMember(record, companyID, `${slug}-sample@example.test`);
	await record.from('member').update({ is_admin: true, status: 'active' }).eq('id', secondAdminID);
	await record.from('member').update({ status: 'active' }).eq('id', sampleID);
}, networkHookTimeout);

afterAll(async () => {
	if (companyID) await record.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('who is asked to act on somebody else behalf', () => {
	test('is every administrator', async () => {
		const asked = await whoAnswersFor(record, companyID, sampleID);

		expect(asked.sort()).toEqual([adminID, secondAdminID].sort());
	});

	test('never asks the person who asked', async () => {
		expect(await whoAnswersFor(record, companyID, secondAdminID)).toEqual([adminID]);
	});
});
