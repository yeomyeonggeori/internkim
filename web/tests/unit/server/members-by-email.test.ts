import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { membersOfCompanyByEmail } from '../../../src/lib/server/member-credential';

type MemberRow = { id: string; email: string | null; status: string };

function aDirectoryHolding(rows: MemberRow[]) {
	const asked: { column: string; value: string }[] = [];
	const client = {
		from() {
			const query = {
				select: () => query,
				eq: (column: string, value: string) => {
					asked.push({ column, value });
					return query;
				},
				neq: (column: string, value: string) => {
					asked.push({ column, value });
					return query;
				},
				returns: () =>
					Promise.resolve({
						data: rows.filter((row) => row.status !== 'withdrawn'),
						error: null
					})
			};
			return query;
		}
	} as unknown as SupabaseClient;
	return { client, asked };
}

describe('membersOfCompanyByEmail', () => {
	test('finds a member who has no messenger account anywhere', async () => {
		const { client } = aDirectoryHolding([{ id: 'member-1', email: 'New1@Example.com', status: 'invited' }]);
		const memberOf = await membersOfCompanyByEmail(client, 'company-1');
		expect(memberOf.get('new1@example.com')).toBe('member-1');
	});

	test('leaves out whoever no longer works here', async () => {
		const { client } = aDirectoryHolding([
			{ id: 'member-1', email: 'stays@example.com', status: 'active' },
			{ id: 'member-2', email: 'left@example.com', status: 'withdrawn' }
		]);
		const memberOf = await membersOfCompanyByEmail(client, 'company-1');
		expect(memberOf.has('left@example.com')).toBe(false);
		expect(memberOf.get('stays@example.com')).toBe('member-1');
	});

	test('leaves out a member the directory holds no address for', async () => {
		const { client } = aDirectoryHolding([{ id: 'member-1', email: null, status: 'active' }]);
		const memberOf = await membersOfCompanyByEmail(client, 'company-1');
		expect(memberOf.size).toBe(0);
	});
});
