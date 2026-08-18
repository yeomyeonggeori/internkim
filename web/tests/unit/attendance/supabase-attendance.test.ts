import { describe, expect, test } from 'bun:test';
import {
	supabaseAttendanceAbsences,
	type SupabaseAttendanceLeave
} from '../../../src/lib/attendance/supabase-attendance';

const member = {
	id: 'member-one',
	name: '이샘플',
	email: 'sample@example.com',
	is_admin: false,
	user_id: 'user-one',
	joined_at: '2026-01-01T00:00:00Z'
};

function leave(overrides: Partial<SupabaseAttendanceLeave> = {}): SupabaseAttendanceLeave {
	return {
		id: 'leave-one',
		member_id: member.id,
		kind: 'leave',
		is_paid: true,
		days: 1,
		starts_at: '2026-08-10T15:00:00Z',
		ends_at: '2026-08-11T15:00:00Z',
		note: null,
		cancelled_at: null,
		...overrides
	};
}

describe('supabaseAttendanceAbsences', () => {
	test('projects an exclusive full-day range onto one absence date', () => {
		const absences = supabaseAttendanceAbsences(leave(), member, 'Asia/Seoul');

		expect(absences.map((absence) => absence.date)).toEqual(['2026-08-11']);
		expect(absences[0].startDate).toBe('2026-08-11');
		expect(absences[0].endDate).toBe('2026-08-11');
	});

	test('does not project a cancelled approved leave', () => {
		const absences = supabaseAttendanceAbsences(
			leave({ cancelled_at: '2026-08-01T00:00:00Z' }),
			member,
			'Asia/Seoul'
		);

		expect(absences).toEqual([]);
	});
});
