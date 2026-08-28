import { isAttendanceWorkMode, type AttendanceWorkMode } from './work-mode';

export type CurrentAttendanceWorkPolicy = {
	workMode: AttendanceWorkMode;
	workingWeekdays: number[];
	dailyTargetMinutes: number;
	weeklyTargetMinutes: number;
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

const policyKeys = [
	'workMode',
	'workingWeekdays',
	'dailyTargetMinutes',
	'weeklyTargetMinutes',
	'referenceStartTime',
	'fixedStartTime',
	'fixedEndTime',
	'coreTimeEnabled',
	'coreStartTime',
	'coreEndTime',
	'breakPeriods',
	'nightStartTime',
	'nightEndTime'
] as const;

export function currentAttendanceWorkPolicy(value: unknown): CurrentAttendanceWorkPolicy {
	if (typeof value !== 'object' || value === null) {
		throw new Error('workPolicy must be an object');
	}
	const offered = value as Record<string, unknown>;
	if (!policyKeys.every((key) => key in offered)) {
		throw new Error('workPolicy is missing required fields');
	}
	if (!isAttendanceWorkMode(offered.workMode)) {
		throw new Error('workPolicy workMode is invalid');
	}
	if (
		!Array.isArray(offered.workingWeekdays) ||
		offered.workingWeekdays.length === 0 ||
		!offered.workingWeekdays.every(
			(day) => Number.isInteger(day) && Number(day) >= 1 && Number(day) <= 7
		) ||
		new Set(offered.workingWeekdays).size !== offered.workingWeekdays.length
	) {
		throw new Error('workPolicy workingWeekdays are invalid');
	}
	if (!isNonNegativeInteger(offered.dailyTargetMinutes)) {
		throw new Error('workPolicy dailyTargetMinutes is invalid');
	}
	if (!isNonNegativeInteger(offered.weeklyTargetMinutes)) {
		throw new Error('workPolicy weeklyTargetMinutes is invalid');
	}
	if (typeof offered.coreTimeEnabled !== 'boolean') {
		throw new Error('workPolicy coreTimeEnabled is invalid');
	}
	const timeValues = {} as Record<string, string>;
	for (const key of [
		'referenceStartTime',
		'fixedStartTime',
		'fixedEndTime',
		'coreStartTime',
		'coreEndTime',
		'nightStartTime',
		'nightEndTime'
	] as const) {
		if (typeof offered[key] !== 'string') throw new Error(`workPolicy ${key} is invalid`);
		timeValues[key] = offered[key];
	}
	if (!Array.isArray(offered.breakPeriods) || !offered.breakPeriods.every(isBreakPeriod)) {
		throw new Error('workPolicy breakPeriods are invalid');
	}
	const breakPeriods = normalizedBreakPeriods(offered.breakPeriods);
	validatePolicySchedule(offered, timeValues, breakPeriods);
	return {
		workMode: offered.workMode,
		workingWeekdays: [...new Set(offered.workingWeekdays as number[])].sort(
			(left, right) => left - right
		),
		dailyTargetMinutes: offered.dailyTargetMinutes,
		weeklyTargetMinutes: offered.weeklyTargetMinutes,
		referenceStartTime: offered.referenceStartTime as string,
		fixedStartTime: offered.fixedStartTime as string,
		fixedEndTime: offered.fixedEndTime as string,
		coreTimeEnabled: offered.coreTimeEnabled,
		coreStartTime: offered.coreStartTime as string,
		coreEndTime: offered.coreEndTime as string,
		breakPeriods,
		nightStartTime: offered.nightStartTime as string,
		nightEndTime: offered.nightEndTime as string
	};
}

function isNonNegativeInteger(value: unknown): value is number {
	return Number.isInteger(value) && Number(value) >= 0;
}

function isBreakPeriod(value: unknown): value is { startTime: string; endTime: string } {
	return (
		typeof value === 'object' &&
		value !== null &&
		'startTime' in value &&
		typeof value.startTime === 'string' &&
		'endTime' in value &&
		typeof value.endTime === 'string'
	);
}

function normalizedBreakPeriods(
	periods: { startTime: string; endTime: string }[]
): { startTime: string; endTime: string }[] {
	const normalized = periods
		.map((period) => ({ ...period }))
		.sort((left, right) => requiredClockMinute(left.startTime, 'breakPeriods') - requiredClockMinute(right.startTime, 'breakPeriods'));
	for (let index = 0; index < normalized.length; index += 1) {
		const period = normalized[index];
		if (!period) continue;
		const start = requiredClockMinute(period.startTime, 'breakPeriods');
		const end = requiredClockMinute(period.endTime, 'breakPeriods');
		if (start >= end) throw new Error('workPolicy breakPeriods are invalid');
		const previous = normalized[index - 1];
		if (previous && requiredClockMinute(previous.endTime, 'breakPeriods') > start) {
			throw new Error('workPolicy breakPeriods overlap');
		}
	}
	return normalized;
}

function validatePolicySchedule(
	offered: Record<string, unknown>,
	times: Record<string, string>,
	breakPeriods: { startTime: string; endTime: string }[]
): void {
	const workMode = offered.workMode as AttendanceWorkMode;
	const dailyTargetMinutes = offered.dailyTargetMinutes as number;
	const weeklyTargetMinutes = offered.weeklyTargetMinutes as number;
	const weekdays = offered.workingWeekdays as number[];
	if (dailyTargetMinutes >= 24 * 60 || (workMode !== 'autonomous' && dailyTargetMinutes === 0)) {
		throw new Error('workPolicy dailyTargetMinutes is invalid');
	}
	const expectedWeeklyMinutes = workMode === 'autonomous' ? 0 : dailyTargetMinutes * weekdays.length;
	if (weeklyTargetMinutes !== expectedWeeklyMinutes) {
		throw new Error('workPolicy weeklyTargetMinutes is invalid');
	}
	requiredClockMinute(times.referenceStartTime ?? '', 'referenceStartTime');
	const nightStart = requiredClockMinute(times.nightStartTime ?? '', 'nightStartTime');
	const nightEnd = requiredClockMinute(times.nightEndTime ?? '', 'nightEndTime');
	if (nightStart === nightEnd) throw new Error('workPolicy night hours are invalid');

	if (workMode === 'fixed') {
		const fixedStart = requiredClockMinute(times.fixedStartTime ?? '', 'fixedStartTime');
		const fixedEnd = requiredClockMinute(times.fixedEndTime ?? '', 'fixedEndTime');
		if (fixedStart >= fixedEnd) throw new Error('workPolicy fixed hours are invalid');
		const breakMinutes = breakPeriods.reduce(
			(total, period) => total + rangeOverlap(
				fixedStart,
				fixedEnd,
				requiredClockMinute(period.startTime, 'breakPeriods'),
				requiredClockMinute(period.endTime, 'breakPeriods')
			),
			0
		);
		if (fixedEnd - fixedStart - breakMinutes !== dailyTargetMinutes) {
			throw new Error('workPolicy fixed hours do not match dailyTargetMinutes');
		}
		if (offered.coreTimeEnabled || times.coreStartTime || times.coreEndTime) {
			throw new Error('workPolicy core hours are invalid for fixed work');
		}
		return;
	}
	if (times.fixedStartTime || times.fixedEndTime) {
		throw new Error(`workPolicy fixed hours are invalid for ${workMode} work`);
	}
	if (workMode === 'autonomous') {
		if (offered.coreTimeEnabled || times.coreStartTime || times.coreEndTime) {
			throw new Error('workPolicy core hours are invalid for autonomous work');
		}
		return;
	}
	if (!offered.coreTimeEnabled) {
		if (times.coreStartTime || times.coreEndTime) {
			throw new Error('workPolicy core hours are invalid');
		}
		return;
	}
	const coreStart = requiredClockMinute(times.coreStartTime ?? '', 'coreStartTime');
	const coreEnd = requiredClockMinute(times.coreEndTime ?? '', 'coreEndTime');
	if (coreStart >= coreEnd) throw new Error('workPolicy core hours are invalid');
}

export function requiredClockMinute(value: string, field: string): number {
	if (!/^\d{2}:\d{2}$/.test(value)) throw new Error(`workPolicy ${field} is invalid`);
	const [hour, minute] = value.split(':').map(Number);
	if (hour > 23 || minute > 59) throw new Error(`workPolicy ${field} is invalid`);
	return hour * 60 + minute;
}

function rangeOverlap(leftStart: number, leftEnd: number, rightStart: number, rightEnd: number): number {
	return Math.max(0, Math.min(leftEnd, rightEnd) - Math.max(leftStart, rightStart));
}
