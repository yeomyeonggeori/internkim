import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { removeMember } from '../../../src/lib/server/control-plane';

type Attempt = { kind: 'delete' | 'update'; row?: Record<string, unknown> };

function aRecordThat(options: { refusesDelete?: boolean } = {}) {
	const attempts: Attempt[] = [];
	const client = {
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
			return {
				delete: () => answer({ kind: 'delete' }),
				update: (row: Record<string, unknown>) => answer({ kind: 'update', row })
			};
		}
	} as unknown as SupabaseClient;
	return { client, attempts };
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

	test('keeps the row a record still names, even when asked to erase it', async () => {
		const { client, attempts } = aRecordThat({ refusesDelete: true });
		const outcome = await removeMember(client, 'company-1', 'member-1', { purge: true });
		expect(outcome.wasRemoved).toBe(false);
		expect(attempts.map((attempt) => attempt.kind)).toEqual(['delete', 'update']);
	});
});
