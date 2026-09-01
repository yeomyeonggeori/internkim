export type DueEvent = {
	id: string;
	title: string;
	startsAt: string;
	isWholeDay: boolean;
	notifyMinutesBefore: number | null;
};

const oneMinuteInMilliseconds = 60_000;

export function minuteOf(moment: Date): number {
	return Math.floor(moment.getTime() / oneMinuteInMilliseconds);
}

export function remindsAt(event: DueEvent): Date | null {
	if (event.notifyMinutesBefore === null) return null;
	if (event.notifyMinutesBefore <= 0) return null;
	const start = new Date(event.startsAt);
	if (Number.isNaN(start.getTime())) return null;
	return new Date(start.getTime() - event.notifyMinutesBefore * oneMinuteInMilliseconds);
}

export function isDue(event: DueEvent, moment: Date): boolean {
	const reminder = remindsAt(event);
	if (reminder === null) return false;
	if (new Date(event.startsAt).getTime() <= moment.getTime()) return false;
	return minuteOf(reminder) === minuteOf(moment);
}

export function startsIn(event: DueEvent, moment: Date): number {
	return Math.round((new Date(event.startsAt).getTime() - moment.getTime()) / oneMinuteInMilliseconds);
}
