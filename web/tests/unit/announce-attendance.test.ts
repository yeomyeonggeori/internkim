import { afterEach, describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import {
	announceClock,
	announceLeaveRequest,
	clockBody,
	leaveBody,
	zoneOf,
	type Member
} from '../../src/lib/server/announce-attendance';

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
		single: async () => ({
			data: {
				id: 'me',
				name: '이샘플',
				company_id: 'company-1',
				timezone: null,
				company: { timezone: 'Asia/Seoul' }
			},
			error: null
		}),
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
				{ id: 'member-1', is_admin: false }
			]
		});

		await announceLeaveRequest(client, client, 'me', vapid, 1_700_000_000);

		expect(told).toEqual(['admin-1']);
	});

	test('says nothing when no request is waiting', async () => {
		const { client, told } = planeHolding({ leave: null, members: [{ id: 'admin-1', is_admin: true }] });
		expect(await announceLeaveRequest(client, client, 'me', vapid, 1_700_000_000)).toEqual({ told: 0, reached: 0 });
		expect(told).toEqual([]);
	});
});

describe('writing a clock body in the company time zone', () => {
	test('reads the hour the company keeps, not the one the server runs in', () => {
		const clocked = { id: 'a1', kind: 'clock_in', location: '본사', occurred_at: '2026-09-03T00:05:00Z' };
		expect(clockBody(clocked, 'Asia/Seoul')).toBe('본사 · 09:05');
	});

	test('an hour after the company midnight is not the hour before it', () => {
		const clocked = { id: 'a1', kind: 'clock_in', location: null, occurred_at: '2026-09-02T15:30:00Z' };
		expect(clockBody(clocked, 'Asia/Seoul')).toBe('00:30');
	});

	test('a company west of UTC reads its own hour too', () => {
		const clocked = { id: 'a1', kind: 'clock_out', location: null, occurred_at: '2026-09-03T22:30:00Z' };
		expect(clockBody(clocked, 'America/New_York')).toBe('18:30');
	});
});

describe('writing a leave body in the company time zone', () => {
	test('a whole day off is that day, not the day before it', () => {
		const asked = { id: 'l1', kind: '연차', starts_at: '2026-09-02T15:00:00Z', ends_at: '2026-09-03T15:00:00Z' };
		expect(leaveBody(asked, 'Asia/Seoul')).toBe('연차 2026-09-03');
	});

	test('several days off end on the last day off, not the morning after', () => {
		const asked = { id: 'l1', kind: '연차', starts_at: '2026-09-02T15:00:00Z', ends_at: '2026-09-04T15:00:00Z' };
		expect(leaveBody(asked, 'Asia/Seoul')).toBe('연차 2026-09-03 ~ 2026-09-04');
	});

	test('a request that ends where it starts is one day, not a range running backwards', () => {
		const asked = { id: 'l1', kind: '연차', starts_at: '2026-09-02T15:00:00Z', ends_at: '2026-09-02T15:00:00Z' };
		expect(leaveBody(asked, 'Asia/Seoul')).toBe('연차 2026-09-03');
	});

	test('a request with no end is the day it starts', () => {
		const asked = { id: 'l1', kind: '반차', starts_at: '2026-09-02T15:00:00Z', ends_at: null };
		expect(leaveBody(asked, 'Asia/Seoul')).toBe('반차 2026-09-03');
	});
});

describe('choosing whose clock the announcement reads', () => {
	const working = (timezone: string | null, company: { timezone: string } | null): Member => ({
		id: 'me',
		name: '이샘플',
		company_id: 'company-1',
		timezone,
		company
	});

	test('a member who keeps their own time zone keeps it', () => {
		expect(zoneOf(working('America/New_York', { timezone: 'Asia/Seoul' }))).toBe('America/New_York');
	});

	test('a member who keeps none reads the company clock', () => {
		expect(zoneOf(working(null, { timezone: 'Asia/Seoul' }))).toBe('Asia/Seoul');
	});

	test('a member with no clock anywhere is refused rather than guessed', () => {
		expect(() => zoneOf(working(null, null))).toThrow('me');
	});

	test('a whole day off written at the member midnight reads as that one day', () => {
		const zone = zoneOf(working('America/New_York', { timezone: 'Asia/Seoul' }));
		const asked = { id: 'l1', kind: '연차', starts_at: '2026-09-03T04:00:00Z', ends_at: '2026-09-04T04:00:00Z' };
		expect(leaveBody(asked, zone)).toBe('연차 2026-09-03');
	});
});
