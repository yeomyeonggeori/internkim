const MINUTES_PER_HOUR = 60;

export function formatHoursMinutes(minutes: number): string {
	if (!minutes) return '0h';
	const hours = Math.floor(minutes / MINUTES_PER_HOUR);
	const remainder = minutes % MINUTES_PER_HOUR;
	if (!hours) return `${remainder}m`;
	if (!remainder) return `${hours}h`;
	return `${hours}h ${remainder}m`;
}

export function formatTimeOfDay(minutesSinceMidnight: number): string {
	const hh = Math.floor(minutesSinceMidnight / 60) % 24;
	const mm = Math.round(minutesSinceMidnight % 60);
	return `${hh.toString().padStart(2, '0')}:${mm.toString().padStart(2, '0')}`;
}
