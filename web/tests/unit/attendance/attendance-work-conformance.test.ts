import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';
import {
	calculateSupabaseEmployeeWorkStatus,
	type SupabaseWorkStatusAttendance,
	type SupabaseWorkStatusLeave
} from '../../../src/lib/attendance/supabase-work-status';
import { companyTimeInstant, shiftedDay } from '../../../src/lib/attendance/supabase-work-status-range';

type AttendanceWorkConformancePolicy = {
	workMode: 'autonomous' | 'flexible' | 'fixed';
	workingWeekdays: number[];
	dailyTargetMinutes: number;
	referenceStartTime: string;
	fixedStartTime: string;
	fixedEndTime: string;
	coreTimeEnabled: boolean;
	coreStartTime: string;
	coreEndTime: string;
	breakPeriods: { startTime: string; endTime: string }[];
	nightStartTime: string;
	nightEndTime: string;
};

type AttendanceWorkConformanceEvent = {
	kind: 'clock_in' | 'clock_out';
	occurredAt: string;
	location?: string;
};

type AttendanceWorkConformanceLeave = {
	startDate: string;
	endDate: string;
	days: number;
};

type AttendanceWorkConformanceExpected = {
	actualSeconds: number;
	actualMinutes: number;
	provisionalSeconds: number;
	provisionalMinutes: number;
	targetMinutes: number;
	leaveMinutes: number;
	nightMinutes: number;
	hasBaseline: boolean;
	hasIncompleteRecords: boolean;
	needsReview: boolean;
};

type AttendanceWorkConformanceScenario = {
	name: string;
	timeZone: string;
	now: string;
	periodStart: string;
	periodEnd: string;
	policy: AttendanceWorkConformancePolicy;
	events: AttendanceWorkConformanceEvent[];
	leave: AttendanceWorkConformanceLeave[];
	expected: AttendanceWorkConformanceExpected;
};

type AttendanceWorkConformanceFile = {
	scenarios: AttendanceWorkConformanceScenario[];
};

const conformanceFilePath = join(
	import.meta.dir,
	'../../../../internal/admind/testdata/attendance-work-conformance.json'
);

function readConformanceFile(): AttendanceWorkConformanceFile {
	const raw = readFileSync(conformanceFilePath, 'utf-8');
	return JSON.parse(raw) as AttendanceWorkConformanceFile;
}

const conformanceMember = {
	id: 'conformance-member',
	name: '이샘플',
	email: 'conformance@example.com',
	is_admin: false,
	user_id: 'conformance-user',
	joined_at: '2026-01-01T00:00:00Z'
};

function conformanceDays(periodStart: string, periodEnd: string): string[] {
	const days: string[] = [];
	let day = periodStart;
	while (day <= periodEnd) {
		days.push(day);
		day = shiftedDay(day, 1);
	}
	return days;
}

function conformancePolicy(source: AttendanceWorkConformancePolicy) {
	const weeklyTargetMinutes =
		source.workMode === 'autonomous' ? 0 : source.dailyTargetMinutes * source.workingWeekdays.length;
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: conformanceMember.id,
			work_hours: null,
			minimum_daily_minutes: source.dailyTargetMinutes,
			work_mode: source.workMode,
			work_calendar: null,
			work_policy: {
				workMode: source.workMode,
				workingWeekdays: source.workingWeekdays,
				dailyTargetMinutes: source.dailyTargetMinutes,
				weeklyTargetMinutes,
				referenceStartTime: source.referenceStartTime,
				fixedStartTime: source.fixedStartTime,
				fixedEndTime: source.fixedEndTime,
				coreTimeEnabled: source.coreTimeEnabled,
				coreStartTime: source.coreStartTime,
				coreEndTime: source.coreEndTime,
				breakPeriods: source.breakPeriods,
				nightStartTime: source.nightStartTime,
				nightEndTime: source.nightEndTime
			}
		}
	]);
	const policy = policies.get(conformanceMember.id);
	if (!policy) throw new Error('expected conformance work policy fixture');
	return policy;
}

function conformanceEvents(
	scenario: AttendanceWorkConformanceScenario
): SupabaseWorkStatusAttendance[] {
	return scenario.events.map((event) => {
		if (event.kind !== 'clock_in' && event.kind !== 'clock_out') {
			throw new Error(`scenario ${scenario.name}: unknown conformance event kind ${event.kind}`);
		}
		return {
			member_id: conformanceMember.id,
			kind: event.kind,
			occurred_at: event.occurredAt,
			location: event.location ?? null
		};
	});
}

function conformanceLeave(scenario: AttendanceWorkConformanceScenario): SupabaseWorkStatusLeave[] {
	return scenario.leave.map((leave) => {
		if (leave.days > 0.5) {
			return {
				member_id: conformanceMember.id,
				days: leave.days,
				starts_at: companyTimeInstant(leave.startDate, '00:00', scenario.timeZone),
				ends_at: companyTimeInstant(shiftedDay(leave.endDate, 1), '00:00', scenario.timeZone),
				status: 'approved'
			};
		}
		if (leave.startDate !== leave.endDate) {
			throw new Error(
				`scenario ${scenario.name}: partial-day leave startDate must equal endDate`
			);
		}
		return {
			member_id: conformanceMember.id,
			days: leave.days,
			starts_at: companyTimeInstant(leave.startDate, '09:00', scenario.timeZone),
			ends_at: companyTimeInstant(leave.startDate, '14:00', scenario.timeZone),
			status: 'approved'
		};
	});
}

describe('attendance work conformance', () => {
	const file = readConformanceFile();

	test('the shared scenario file is non-empty', () => {
		expect(file.scenarios.length > 0).toBe(true);
	});

	let executed = 0;
	for (const scenario of file.scenarios) {
		test(scenario.name, () => {
			executed += 1;
			const policy = conformancePolicy(scenario.policy);
			const days = conformanceDays(scenario.periodStart, scenario.periodEnd);
			const status = calculateSupabaseEmployeeWorkStatus({
				member: conformanceMember,
				days,
				timeZone: scenario.timeZone,
				attendance: conformanceEvents(scenario),
				leave: conformanceLeave(scenario),
				policy,
				now: new Date(scenario.now)
			});

			expect(status.actualSeconds).toBe(scenario.expected.actualSeconds);
			expect(status.actualMinutes).toBe(scenario.expected.actualMinutes);
			expect(status.provisionalSeconds).toBe(scenario.expected.provisionalSeconds);
			expect(status.provisionalMinutes).toBe(scenario.expected.provisionalMinutes);
			expect(status.targetMinutes).toBe(scenario.expected.targetMinutes);
			expect(status.leaveMinutes).toBe(scenario.expected.leaveMinutes);
			expect(status.nightMinutes).toBe(scenario.expected.nightMinutes);
			expect(status.hasBaseline).toBe(scenario.expected.hasBaseline);
			expect(status.hasIncompleteRecords).toBe(scenario.expected.hasIncompleteRecords);
			expect(status.needsReview).toBe(scenario.expected.needsReview);
		});
	}

	test('every scenario in the file was executed', () => {
		expect(executed).toBe(file.scenarios.length);
	});
});
