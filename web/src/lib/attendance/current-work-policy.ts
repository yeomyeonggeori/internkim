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
		!offered.workingWeekdays.every(
			(day) => Number.isInteger(day) && Number(day) >= 0 && Number(day) <= 6
		)
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
	}
	if (!Array.isArray(offered.breakPeriods) || !offered.breakPeriods.every(isBreakPeriod)) {
		throw new Error('workPolicy breakPeriods are invalid');
	}
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
		breakPeriods: offered.breakPeriods.map((period) => ({ ...period })) as {
			startTime: string;
			endTime: string;
		}[],
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
