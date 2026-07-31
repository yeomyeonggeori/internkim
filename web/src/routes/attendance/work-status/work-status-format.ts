import { formatHoursMinutes } from '../shared/attendance-format';

type WorkStatusDurationUnits = {
	hourUnit: string;
	minuteUnit: string;
};

const millisecondsPerDay = 24 * 60 * 60 * 1000;
const minutesPerDay = 24 * 60;

export function calculatePeriodCapacityMinutes(
	periodStart: string,
	periodEnd: string
): number {
	const startMilliseconds = Date.parse(`${periodStart}T00:00:00Z`);
	const endMilliseconds = Date.parse(`${periodEnd}T00:00:00Z`);
	return (
		(Math.floor((endMilliseconds - startMilliseconds) / millisecondsPerDay) + 1) *
		minutesPerDay
	);
}

export function formatWorkStatusDuration(
	minutes: number,
	units: WorkStatusDurationUnits
): string {
	return (
		formatHoursMinutes(minutes, units) ||
		`00${units.hourUnit} 00${units.minuteUnit}`
	);
}
