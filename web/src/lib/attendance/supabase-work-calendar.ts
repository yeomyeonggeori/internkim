const millisecondsPerDay = 24 * 60 * 60 * 1000;
const cycleAnchorMilliseconds = Date.parse('2000-01-03T00:00:00Z');

export type WorkHoursCycle = unknown[][] | null;

export function isScheduledWorkingDate(
	day: string,
	workHours: WorkHoursCycle
): boolean {
	if (workHours === null) return true;
	if (workHours.length === 0 || workHours.some((week) => week.length !== 7)) {
		throw new Error('resolved work hours must contain complete seven-day weeks');
	}
	const dayMilliseconds = Date.parse(`${day}T00:00:00Z`);
	if (Number.isNaN(dayMilliseconds)) throw new Error(`invalid work status date: ${day}`);
	const daysSinceAnchor = Math.floor(
		(dayMilliseconds - cycleAnchorMilliseconds) / millisecondsPerDay
	);
	const weekIndex = positiveModulo(Math.floor(daysSinceAnchor / 7), workHours.length);
	const weekdayIndex = positiveModulo(daysSinceAnchor, 7);
	return workHours[weekIndex][weekdayIndex] !== null;
}

function positiveModulo(value: number, divisor: number): number {
	return ((value % divisor) + divisor) % divisor;
}
