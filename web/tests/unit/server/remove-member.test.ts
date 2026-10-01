import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { removeMember } from '../../../src/lib/server/control-plane';

type Attempt = { kind: 'delete' | 'update'; row?: Record<string, unknown> };

type BanAsked = { accountID: string; banDuration: unknown };

function aRecordThat(options: { refusesDelete?: boolean; accountID?: string } = {}) {
	const attempts: Attempt[] = [];
	const bans: BanAsked[] = [];
	const heldAccount = { data: { user_id: options.accountID ?? null }, error: null };
	const client = {
		auth: {
			admin: {
				updateUserById: async (accountID: string, attributes: { ban_duration?: unknown }) => {
					bans.push({ accountID, banDuration: attributes.ban_duration });
					return { error: null };
				}
			}
		},
		from() {
			const answer = (attempt: Attempt) => {
				const refused =
					attempt.kind === 'delete' && options.refusesDelete
						? { code: '23503', message: 'attendance still names them' }
						: null;
				const query = {
					eq: () => query,
					then: (resolve: (value: { error: unknown }) => unknown) => resolve({ error: refused })
				};
				attempts.push(attempt);
				return query;
			};
			const lookup = { eq: () => lookup, maybeSingle: async () => heldAccount };
			return {
				select: () => lookup,
				delete: () => answer({ kind: 'delete' }),
				update: (row: Record<string, unknown>) => answer({ kind: 'update', row })
			};
		}
	} as unknown as SupabaseClient;
	return { client, attempts, bans };
}

describe('removeMember', () => {
	test('marks somebody as having left, and keeps the row', async () => {
		const { client, attempts } = aRecordThat();
		const outcome = await removeMember(client, 'company-1', 'member-1');
		expect(outcome.wasRemoved).toBe(false);
		expect(attempts.map((attempt) => attempt.kind)).toEqual(['update']);
		expect(attempts[0].row).toEqual({ status: 'withdrawn' });
	});

	test('erases the row when asked to', async () => {
		const { client, attempts } = aRecordThat();
		const outcome = await removeMember(client, 'company-1', 'member-1', { purge: true });
		expect(outcome.wasRemoved).toBe(true);
		expect(attempts.map((attempt) => attempt.kind)).toEqual(['delete']);
	});

	test('stops the account of somebody who left from signing in again', async () => {
		const { client, bans } = aRecordThat({ accountID: 'account-1' });
		await removeMember(client, 'company-1', 'member-1');
		expect(bans).toEqual([{ accountID: 'account-1', banDuration: '876000h' }]);
	});

	test('stops the account of an erased row from signing in again', async () => {
		const { client, bans } = aRecordThat({ accountID: 'account-1' });
		await removeMember(client, 'company-1', 'member-1', { purge: true });
		expect(bans).toEqual([{ accountID: 'account-1', banDuration: '876000h' }]);
	});

	test('keeps the row a record still names, even when asked to erase it', async () => {
		const { client, attempts } = aRecordThat({ refusesDelete: true });
		const outcome = await removeMember(client, 'company-1', 'member-1', { purge: true });
		expect(outcome.wasRemoved).toBe(false);
		expect(attempts.map((attempt) => attempt.kind)).toEqual(['delete', 'update']);
	});
});
