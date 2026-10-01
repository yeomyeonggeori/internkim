import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { controlPlane, provisionCompany } from '../../src/lib/server/control-plane';
import { leaveKindsOfPolicy } from '../../src/lib/server/public-api/record/leave';
import { projectURL, serviceRoleKey } from './supabase-environment';

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `leave-kind-terms-${Date.now()}`;
let companyID = '';
let memberID = '';

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Leave Kind Terms', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}@example.test`
	);
	companyID = provisioned.companyID;
	memberID = provisioned.adminMemberID;
}, networkHookTimeout);

afterAll(async () => {
	if (companyID) await client.from('company').delete().eq('id', companyID);
}, networkHookTimeout);

describe('a company that stores no leave policy', () => {
	test('the record gives each default kind the pay and deduction the app shows for it', async () => {
		const { data: company } = await client.from('company').select('rules').eq('id', companyID).single();
		expect(company?.rules?.attendanceLeavePolicy).toBeUndefined();

		for (const [index, kind] of leaveKindsOfPolicy(null).entries()) {
			const day = `2031-01-${String(index + 10).padStart(2, '0')}`;
			const { data, error } = await client
				.from('leave')
				.insert({
					member_id: memberID,
					kind: kind.id,
					is_paid: !kind.isPaid,
					is_deducted: !kind.isDeducted,
					days: -1,
					status: 'requested',
					starts_at: `${day}T00:00:00+09:00`,
					ends_at: `${day}T23:59:00+09:00`
				})
				.select('kind, is_paid, is_deducted')
				.single();
			if (error) throw new Error(`${kind.id}: ${error.message}`);
			expect(data).toEqual({ kind: kind.id, is_paid: kind.isPaid, is_deducted: kind.isDeducted });
		}
	});
});
