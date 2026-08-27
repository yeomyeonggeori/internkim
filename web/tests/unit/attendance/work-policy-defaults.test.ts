import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	defaultWorkPolicy,
	initialWorkPolicyEffectiveDate
} from '../../../src/lib/attendance/work-policy-defaults';

const goSource = [
	readFileSync('../internal/admind/attendance_work_policy.go', 'utf8'),
	readFileSync('../internal/admind/attendance_work_schedule.go', 'utf8')
].join('\n');

function goRevisionBody(): string {
	const body = /func\s+defaultAttendanceWorkPolicyRevision\(\)[^{]*\{\s*return\s+attendanceWorkPolicyRevision\{(.*?)\n\t\}\n\}/s.exec(goSource);
	if (!body) throw new Error('defaultAttendanceWorkPolicyRevision is not readable');
	return body[1];
}

function goField(name: string): string {
	const field = new RegExp(`\\b${name}:\\s*([^,\\n]+)`).exec(goRevisionBody());
	if (!field) throw new Error(`${name} is not declared in the Go default revision`);
	return field[1].trim();
}

function goStringField(name: string): string {
	const value = goField(name);
	const quoted = /^"(.*)"$/s.exec(value);
	if (quoted) return quoted[1];
	return goConstant(value);
}

function goMinutesField(name: string): number {
	const value = goField(name);
	const product = /^(\d+)\s*\*\s*(\d+)$/.exec(value);
	if (product) return Number(product[1]) * Number(product[2]);
	if (!/^\d+$/.test(value)) throw new Error(`${name} is not a Go minute count`);
	return Number(value);
}

function goWorkingWeekdays(): number[] {
	const weekdays = /WorkingWeekdays:\s*\[\]int\{([^}]*)\}/.exec(goRevisionBody());
	if (!weekdays) throw new Error('WorkingWeekdays is not declared in the Go default revision');
	return weekdays[1].split(',').map((day) => Number(day.trim()));
}

function goBreakPeriods(): { startTime: string; endTime: string }[] {
	const periods = /BreakPeriods:\s*\[\]attendanceWorkScheduleBreakPeriod\{(.*?)\n\t\t\}/s.exec(goRevisionBody());
	if (!periods) throw new Error('BreakPeriods is not declared in the Go default revision');
	const times = [...periods[1].matchAll(/StartTime:\s*"([^"]*)",\s*\n?\s*EndTime:\s*"([^"]*)"/g)];
	return times.map((period) => ({ startTime: period[1], endTime: period[2] }));
}

function goConstant(name: string): string {
	const declaration = new RegExp(`${name}\\s*=\\s*"([^"]*)"`).exec(goSource);
	if (!declaration) throw new Error(`${name} is not declared`);
	return declaration[1];
}

describe('the central plane answers the device default work policy', () => {
	test('the Go default revision is readable', () => {
		expect(goWorkingWeekdays()).toEqual([1, 2, 3, 4, 5]);
		expect(goBreakPeriods().length).not.toBe(0);
	});

	test('the default work policy matches the one internal/admind declares', () => {
		expect(defaultWorkPolicy()).toEqual({
			workMode: goStringField('WorkMode'),
			workingWeekdays: goWorkingWeekdays(),
			dailyTargetMinutes: goMinutesField('DailyTargetMinutes'),
			weeklyTargetMinutes: goMinutesField('WeeklyTargetMinutes'),
			referenceStartTime: goStringField('ReferenceStartTime'),
			fixedStartTime: goStringField('FixedStartTime'),
			fixedEndTime: goStringField('FixedEndTime'),
			coreTimeEnabled: goField('CoreTimeEnabled') === 'true',
			coreStartTime: goStringField('CoreStartTime'),
			coreEndTime: goStringField('CoreEndTime'),
			breakPeriods: goBreakPeriods(),
			nightStartTime: goStringField('NightStartTime'),
			nightEndTime: goStringField('NightEndTime')
		});
	});

	test('the initial effective date is the one the device stamps', () => {
		expect(initialWorkPolicyEffectiveDate).toBe(
			goConstant('attendanceWorkPolicyInitialEffectiveDate')
		);
	});
});
