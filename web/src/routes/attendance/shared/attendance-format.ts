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
	if (minutes <= 0) return '';
	const hours = Math.floor(minutes / MINUTES_PER_HOUR);
	const remainder = minutes % MINUTES_PER_HOUR;
	return `${padDurationNumber(hours)}${units.hourUnit} ${padDurationNumber(remainder)}${units.minuteUnit}`;
}

export function padDurationNumber(value: number): string {
	return String(value).padStart(2, '0');
}

export function formatTimeOfDay(minutesSinceMidnight: number): string {
	const hh = Math.floor(minutesSinceMidnight / 60) % 24;
	const mm = Math.round(minutesSinceMidnight % 60);
	return `${hh.toString().padStart(2, '0')}:${mm.toString().padStart(2, '0')}`;
}
