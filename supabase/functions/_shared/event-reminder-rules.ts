import type { CentralTaskStatus } from './central-task-status.ts';

export type DueEvent = {
	id: string;
	title: string;
	startsAt: string;
	isWholeDay: boolean;
	notifyMinutesBefore: number | null;
};

export type Participant = { member: { id: string; status: string } | null };

export const statusesStillAhead: CentralTaskStatus[] = ['planned', 'in_progress'];

const activeMember = 'active';
const oneMinuteInMilliseconds = 60_000;
const minutesInAnHour = 60;
const minutesInADay = 60 * 24;

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
	return Math.ceil((new Date(event.startsAt).getTime() - moment.getTime()) / oneMinuteInMilliseconds);
}

export function spellOutMinutes(minutes: number): string {
	if (minutes < minutesInAnHour) return `${minutes}분`;
	if (minutes < minutesInADay) {
		return bothUnits(Math.floor(minutes / minutesInAnHour), '시간', minutes % minutesInAnHour, '분');
	}
	return bothUnits(
		Math.floor(minutes / minutesInADay),
		'일',
		Math.floor((minutes % minutesInADay) / minutesInAnHour),
		'시간'
	);
}

function bothUnits(larger: number, largerUnit: string, smaller: number, smallerUnit: string): string {
	if (smaller === 0) return `${larger}${largerUnit}`;
	return `${larger}${largerUnit} ${smaller}${smallerUnit}`;
}

export function whoStillListens(participants: Participant[]): string[] {
	const standing = (participants ?? [])
		.map((participant) => participant.member)
		.filter((member): member is { id: string; status: string } => member !== null)
		.filter((member) => member.status === activeMember)
		.map((member) => member.id);
	return [...new Set(standing)];
}
