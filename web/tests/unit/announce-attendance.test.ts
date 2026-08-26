import { afterEach, describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { announceClock, announceLeaveRequest } from '../../src/lib/server/announce-attendance';

const vapid = {
	publicKey: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	privateKey: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	subject: 'mailto:support@example.com'
};

type Rows = {
	attendance?: Record<string, unknown> | null;
	leave?: Record<string, unknown> | null;
	members: { id: string; is_admin?: boolean }[];
};

function planeHolding(rows: Rows) {
	const told: string[] = [];
	const chain = (table: string, administratorsOnly: { value: boolean }) => ({
		select: () => chain(table, administratorsOnly),
		eq: (column: string, value: unknown) => {
			if (column === 'is_admin' && value === true) administratorsOnly.value = true;
			return chain(table, administratorsOnly);
		},
		neq: () => chain(table, administratorsOnly),
		order: () => chain(table, administratorsOnly),
		limit: () => chain(table, administratorsOnly),
		maybeSingle: async () => ({ data: table === 'attendance' ? rows.attendance : rows.leave, error: null }),
		single: async () => ({ data: { id: 'me', name: '이샘플', company_id: 'company-1' }, error: null }),
		returns: async () => ({
			data: rows.members.filter((member) => !administratorsOnly.value || member.is_admin),
			error: null
		}),
		delete: () => chain(table, administratorsOnly)
	});

	const client = {
		from(table: string) {
			if (table === 'member') {
				return {
					select: (columns: string) => {
						const administratorsOnly = { value: false };
						// notifyMember reads one member's settings; the announcer reads one row.
						if (columns.includes('notification_settings')) {
							return {
								eq: () => ({ maybeSingle: async () => ({ data: { notification_settings: {} }, error: null }) })
							};
						}
						return chain('member', administratorsOnly);
					}
				};
			}
			if (table === 'push_device') {
				return {
					select: () => ({
						eq: (_column: string, memberID: string) => ({
							eq: () => ({
								returns: async () => {
									told.push(memberID);
									return { data: [], error: null };
								}
							})
						})
					})
				};
			}
			return chain(table, { value: false });
		}
	} as unknown as SupabaseClient;
	return { client, told };
}

const realFetch = globalThis.fetch;
afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('announcing a clock to the company', () => {
	test('tells everyone except the person who clocked', async () => {
		const { client, told } = planeHolding({
			attendance: { id: 'a1', kind: 'clock_in', location: '본사', occurred_at: '2026-09-01T00:02:00Z' },
			members: [{ id: 'me' }, { id: 'other-1' }, { id: 'other-2' }]
		});

		const announced = await announceClock(client, client, 'me', vapid, 1_700_000_000);

		expect(told).toEqual(['other-1', 'other-2']);
		expect(announced.told).toBe(2);
	});

	test('says nothing when there is no clock to announce', async () => {
		const { client, told } = planeHolding({ attendance: null, members: [{ id: 'other-1' }] });
		expect(await announceClock(client, client, 'me', vapid, 1_700_000_000)).toEqual({ told: 0, reached: 0 });
		expect(told).toEqual([]);
	});
});

describe('announcing a leave request', () => {
	test('reaches the administrators and nobody else', async () => {
		const { client, told } = planeHolding({
			leave: { id: 'l1', kind: '연차', starts_at: '2026-09-01T00:00:00Z', ends_at: '2026-09-03T00:00:00Z' },
			members: [
				{ id: 'me', is_admin: true },
				{ id: 'admin-1', is_admin: true },
				{ id: 'staff-1', is_admin: false }
			]
		});

		await announceLeaveRequest(client, client, 'me', vapid, 1_700_000_000);

		// The requester is an administrator here and still hears nothing about
		// their own request.
		expect(told).toEqual(['admin-1']);
	});

	test('says nothing when no request is waiting', async () => {
		const { client, told } = planeHolding({ leave: null, members: [{ id: 'admin-1', is_admin: true }] });
		expect(await announceLeaveRequest(client, client, 'me', vapid, 1_700_000_000)).toEqual({ told: 0, reached: 0 });
		expect(told).toEqual([]);
	});
});
