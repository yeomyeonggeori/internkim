import { describe, expect, mock, test } from 'bun:test';

const databaseServerTime = '2026-08-18T03:04:05.678Z';

function response<Value>(data: Value): Promise<{ data: Value; error: null }> {
	return Promise.resolve({ data, error: null });
}

const client = {
	auth: {
		getSession: () => response({ session: { user: { id: 'account-one' } } })
	},
	from(table: string) {
		if (table === 'company') {
			return {
				select: () => ({ limit: () => ({ single: () => response({ id: 'company-one', timezone: 'Asia/Seoul', work_locations: [], rules: {} }) }) })
			};
		}
		if (table === 'member') {
			return {
				select: () => ({
					neq: () => ({
						returns: () => response([{ id: 'member-one', name: '이샘플', email: 'sample@example.com', is_admin: false, user_id: 'account-one', joined_at: null }])
					})
				})
			};
		}
		if (table === 'attendance') {
			return {
				select: () => ({
					gte: () => ({
						lt: () => ({ order: () => ({ returns: () => response([]) }) })
					})
				})
			};
		}
		return {
			select: () => ({
				eq: () => ({
					is: () => ({
						lt: () => ({ gte: () => ({ returns: () => response([]) }) })
					})
				})
			})
		};
	},
	rpc: (name: string) => {
		if (name === 'attendance_server_time') return response(databaseServerTime);
		if (name === 'attendance_correction_window_minutes') return response(60);
		throw new Error(`Unexpected RPC ${name}`);
	}
};

mock.module('$lib/supabase', () => ({ supabase: () => client }));

const { supabaseAttendanceSummary } = await import('../../../src/lib/attendance/supabase-attendance');

describe('supabaseAttendanceSummary', () => {
	test('uses the database RPC timestamp instead of the browser clock', async () => {
		const browserTime = new Date('2030-01-01T00:00:00.000Z');
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.serverTime).toBe(databaseServerTime);
		expect(summary.serverTime).not.toBe(browserTime.toISOString());
		expect(summary.correctionWindowMinutes).toBe(60);
	});
});
