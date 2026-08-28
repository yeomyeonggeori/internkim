import { requiredClockMinute } from './current-work-policy';

const minutesPerDay = 24 * 60;

export type WorkSchedulePeriod = { startTime: string; endTime: string };

type UpcomingBreak = { startMinute: number; endMinute: number };

export function scheduleEndMinute(
	startMinute: number,
	requiredMinutes: number,
	breakPeriods: WorkSchedulePeriod[]
): number {
	let currentMinute = startMinute;
	let remainingMinutes = requiredMinutes;
	while (remainingMinutes > 0) {
		currentMinute = minuteAfterBreak(currentMinute, breakPeriods);
		const upcoming = nextBreak(currentMinute, breakPeriods);
		const availableMinutes = upcoming
			? upcoming.startMinute - currentMinute
			: minutesPerDay - currentMinute;
		if (remainingMinutes <= availableMinutes) {
			const endMinute = currentMinute + remainingMinutes;
			if (endMinute >= minutesPerDay) {
				throw new Error('the scheduled end time crosses the day boundary');
			}
			return endMinute;
		}
		remainingMinutes -= availableMinutes;
		if (!upcoming) {
			throw new Error('the scheduled end time crosses the day boundary');
		}
		currentMinute = upcoming.endMinute;
	}
	return currentMinute;
}

export function scheduleEndTime(
	startTime: string,
	requiredMinutes: number,
	breakPeriods: WorkSchedulePeriod[]
): string {
	const endMinute = scheduleEndMinute(
		requiredClockMinute(startTime, 'startTime'),
		requiredMinutes,
		breakPeriods
	);
	return clockTime(endMinute);
}

export function clockTime(minute: number): string {
	const hour = Math.floor(minute / 60);
	return `${String(hour).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;
}

function minuteAfterBreak(currentMinute: number, breakPeriods: WorkSchedulePeriod[]): number {
	let minute = currentMinute;
	for (const period of breakPeriods) {
		const startMinute = requiredClockMinute(period.startTime, 'breakPeriods');
		const endMinute = requiredClockMinute(period.endTime, 'breakPeriods');
		if (minute >= startMinute && minute < endMinute) minute = endMinute;
	}
	return minute;
}

function nextBreak(
	currentMinute: number,
	breakPeriods: WorkSchedulePeriod[]
): UpcomingBreak | undefined {
	for (const period of breakPeriods) {
		const startMinute = requiredClockMinute(period.startTime, 'breakPeriods');
		if (startMinute > currentMinute) {
			return { startMinute, endMinute: requiredClockMinute(period.endTime, 'breakPeriods') };
		}
	}
	return undefined;
}
