const MINUTES_PER_HOUR = 60;

type DurationUnitText = {
	hourUnit: string;
	minuteUnit: string;
};

const defaultDurationUnits: DurationUnitText = {
	hourUnit: 'h',
	minuteUnit: 'm',
};

export function formatHoursMinutes(minutes: number, units: DurationUnitText = defaultDurationUnits): string {
	if (!minutes) return `0${units.hourUnit}`;
	const hours = Math.floor(minutes / MINUTES_PER_HOUR);
	const remainder = minutes % MINUTES_PER_HOUR;
	if (!hours) return `${remainder}${units.minuteUnit}`;
	if (!remainder) return `${hours}${units.hourUnit}`;
	return `${hours}${units.hourUnit} ${remainder}${units.minuteUnit}`;
}

export function formatTimeOfDay(minutesSinceMidnight: number): string {
	const hh = Math.floor(minutesSinceMidnight / 60) % 24;
	const mm = Math.round(minutesSinceMidnight % 60);
	return `${hh.toString().padStart(2, '0')}:${mm.toString().padStart(2, '0')}`;
}
