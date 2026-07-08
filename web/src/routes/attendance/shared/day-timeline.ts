const minutesPerDay = 1440;

export function dayWidthPercent(startTime: string, endTime: string): number {
	const startMinutes = clampMinutes(localTimeMinutes(startTime));
	const endMinutes = clampMinutes(Math.max(startMinutes, localTimeMinutes(endTime)));
	return Math.min(100, Math.max(1, ((endMinutes - startMinutes) / minutesPerDay) * 100));
}

export function localTimeMinutes(localTime: string): number {
	const [hours = 0, minutes = 0] = localTime.split(':').map(Number);
	return hours * 60 + minutes;
}

function clampMinutes(minutes: number): number {
	return Math.min(minutesPerDay, Math.max(0, minutes));
}
