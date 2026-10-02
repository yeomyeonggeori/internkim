import { afterEach, beforeEach, describe, expect, mock, spyOn, test } from 'bun:test';
import type { AttendanceSummaryRecords } from '../../../src/lib/attendance/attendance-summary-records';
import * as holidays from '../../../src/lib/attendance/attendance-holidays';

const snapshot: AttendanceSummaryRecords = {
	from: '2026-08-01',
	to: '2026-08-31',
	settings: {
		name: 'Sample', locale: 'en', timeZone: 'Asia/Seoul', currencyCode: 'KRW',
		workLocations: [], leaveDays: 15, teamViewVisibleToAll: true, profileImageURL: null
	},
	directory: {
		requesterID: 'member-one', count: 1,
		people: [{ personID: 'member-one', name: '이샘플', email: 'sample@example.com', isAdmin: false }]
	},
	attendance: {
		from: '2026-07-31', to: '2026-08-31', serverTime: '2026-08-20T00:00:00Z',
		backdatedAfterMinutes: 60, count: 1,
		attendance: [{
			eventID: 'clock-one', personID: 'member-one', person: '이샘플', kind: 'clock_in',
			date: '2026-08-20', time: '09:00', occurredAt: '2026-08-20T00:00:00Z', location: null,
			wasCorrected: false, originalDate: null, originalTime: null, originalOccurredAt: null, reason: null
		}]
	},
	leave: { count: 0, leave: [], registeredKinds: [] }
};

const asked: { name: string; input: Record<string, unknown> }[] = [];
let settingsGate: Promise<void> | undefined;
let attendanceGate: Promise<void> | undefined;
let holidaysRequested = false;

mock.module('../../../src/lib/public-api-call', () => ({
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		asked.push({ name, input });
		if (name === 'company_settings_get') { await settingsGate; return snapshot.settings; }
		if (name === 'person_list') return snapshot.directory;
		if (name === 'attendance_list') { await attendanceGate; return snapshot.attendance; }
		if (name === 'leave_list') return snapshot.leave;
		if (name === 'attendance_work_policy_get') return {
			timeZone: 'Asia/Seoul', workMode: 'flexible', policy: null,
			people: [{ personID: 'member-one', workHours: null, minimumDailyMinutes: 480 }]
		};
		if (name === 'company_holiday_list') return { count: 0, holidays: [] };
		throw new Error(`Unexpected tool ${name}`);
	}
}));
const { supabaseWorkStatusInputs } = await import('../../../src/lib/attendance/supabase-work-status');

describe('attendance summary record reuse', () => {
	beforeEach(() => {
		asked.length = 0;
		settingsGate = undefined;
		attendanceGate = undefined;
		holidaysRequested = false;
		spyOn(holidays, 'attendanceHolidayDates').mockImplementation(async () => {
			holidaysRequested = true;
			return new Set<string>();
		});
	});
	afterEach(() => mock.restore());

	test('derives work-status inputs from the same live monthly records', async () => {
		const result = await supabaseWorkStatusInputs([
			{ period: 'day', anchor: '2026-08-20' }, { period: 'month', anchor: '2026-08-20' }
		], snapshot);
		expect(asked.map((call) => call.name)).toEqual(['attendance_work_policy_get', 'company_holiday_list']);
		expect(result.attendance[0].id).toBe('clock-one');
		expect(result.me?.id).toBe('member-one');
		expect(result.coveredDays[0]).toBe('2026-08-01');
		expect(result.coveredDays.at(-1)).toBe('2026-08-31');
	});

	test('a week crossing the month refetches records for the whole requested range', async () => {
		await supabaseWorkStatusInputs([
			{ period: 'week', anchor: '2026-08-31' }, { period: 'month', anchor: '2026-08-31' }
		], snapshot);
		expect(asked.find((call) => call.name === 'attendance_list')?.input).toEqual({
			scope: 'all', from: '2026-07-31', to: '2026-09-06'
		});
		expect(asked.find((call) => call.name === 'leave_list')?.input).toEqual({
			scope: 'all', status: 'approved', from: '2026-08-01', to: '2026-09-06'
		});
		expect(asked.some((call) => call.name === 'company_settings_get' || call.name === 'person_list')).toBe(false);
	});

	test('a summary without live records revalidates settings, people and attendance', async () => {
		await supabaseWorkStatusInputs([{ period: 'month', anchor: '2026-08-20' }]);
		for (const name of ['company_settings_get', 'person_list', 'attendance_list', 'leave_list']) {
			expect(asked.filter((call) => call.name === name)).toHaveLength(1);
		}
	});

	test('starts directory and policies before settings finishes', async () => {
		let release: (() => void) | undefined;
		settingsGate = new Promise<void>((resolve) => { release = resolve; });
		const reading = supabaseWorkStatusInputs([{ period: 'month', anchor: '2026-08-20' }]);
		expect(asked.map((call) => call.name)).toEqual(['company_settings_get', 'person_list', 'attendance_work_policy_get']);
		if (!release) throw new Error('settings gate was not initialized');
		release();
		await reading;
	});

	test('starts holidays while the attendance response is pending', async () => {
		let release: (() => void) | undefined;
		attendanceGate = new Promise<void>((resolve) => { release = resolve; });
		const reading = supabaseWorkStatusInputs([{ period: 'month', anchor: '2026-09-20' }], snapshot);
		await Promise.resolve();
		await Promise.resolve();
		await Promise.resolve();
		expect(holidaysRequested).toBe(true);
		if (!release) throw new Error('attendance gate was not initialized');
		release();
		await reading;
	});

	test('a refreshed snapshot replaces corrected rows instead of keeping prior records', async () => {
		const request = [{ period: 'month', anchor: '2026-08-20' } satisfies import('../../../src/routes/attendance/attendance-api').AttendanceWorkStatusRequest];
		const first = await supabaseWorkStatusInputs(request, snapshot);
		const corrected = await supabaseWorkStatusInputs(request, {
			...snapshot, attendance: { ...snapshot.attendance, count: 0, attendance: [] }
		});
		expect(first.attendance).toHaveLength(1);
		expect(corrected.attendance).toHaveLength(0);
	});
});
