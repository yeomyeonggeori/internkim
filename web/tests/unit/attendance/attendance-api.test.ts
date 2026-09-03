import { describe, expect, mock, test } from 'bun:test';

const corrected: { corrections: unknown; reason: string }[] = [];

mock.module('../../../src/lib/attendance/supabase-attendance', () => ({
	addSupabaseAttendanceEvent: async () => ({ outcome: 'saved' }),
	correctSupabaseAttendanceEvents: async (corrections: unknown, reason: string) => {
		corrected.push({ corrections, reason });
		return { outcome: 'saved' };
	},
	recordSupabaseAttendance: async () => undefined,
	removeSupabaseAttendanceEvent: async () => ({ outcome: 'saved' }),
	setSupabaseTeamViewVisibility: async () => undefined,
	supabaseAttendanceSummary: async () => ({})
}));

const { updateAttendanceEvents } = await import(
	'../../../src/routes/attendance/attendance-api'
);

function correction(eventID: string, reason: string) {
	return {
		eventID,
		request: { localDate: '2026-08-03', localTime: '09:00', locationID: 'office', reason }
	};
}

describe('updateAttendanceEvents', () => {
	test('sends one correction for every event under the reason they share', async () => {
		corrected.length = 0;

		await updateAttendanceEvents([correction('a', ' 지각 정정 '), correction('b', '지각 정정')]);

		expect(corrected).toHaveLength(1);
		expect(corrected[0].reason).toBe('지각 정정');
		expect(corrected[0].corrections).toEqual([
			{ eventID: 'a', localDate: '2026-08-03', localTime: '09:00', locationID: 'office' },
			{ eventID: 'b', localDate: '2026-08-03', localTime: '09:00', locationID: 'office' }
		]);
	});

	test('refuses corrections that do not agree on why the records were wrong', async () => {
		await expect(
			updateAttendanceEvents([correction('a', '지각 정정'), correction('b', '조퇴 정정')])
		).rejects.toThrow('attendance corrections must share one reason');
	});

	test('refuses an empty correction', async () => {
		await expect(updateAttendanceEvents([])).rejects.toThrow(
			'at least one attendance correction is required'
		);
	});
});
