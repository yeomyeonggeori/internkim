// The importer that brought the device's flow board across and whatever writes a
// later change read a task here, the way both read an event in
// calendar-event-as-task.ts.

import type { DeviceCalendarParticipant, EventCalendar, EventMirror } from './calendar-event-as-task';

export type DeviceFlowTask = {
	id?: string;
	ownerID?: string;
	ownerName?: string;
	participantIDs?: string[];
	participantNames?: string[];
	business?: string;
	type?: string;
	size?: string;
	content?: string;
	goal?: string;
	status?: string;
	startDate?: string;
	endDate?: string;
	requestReason?: string;
};

export type FlowTaskAsTask = {
	title: string;
	note: string | null;
	business: string | null;
	type: string | null;
	size: string | null;
	status: string;
	startsAt: string | null;
	endsAt: string | null;
	isWholeDay: boolean;
	calendar: EventCalendar;
};

const deviceSource = 'internkim-device';

const untitledTask = '(제목 없음)';

// The device's own words for a status, which are what its API returns and what
// its board draws. The enum has one for each of them.
const statusOfDevice: Record<string, string> = {
	요청: 'requested',
	예정: 'todo',
	진행: 'in_progress',
	완료: 'done',
	일시정지: 'paused',
	기각: 'rejected',
	중단: 'cancelled'
};

export function titleOfFlowTask(task: DeviceFlowTask): string {
	return task.content?.trim() || untitledTask;
}

// The owner and the participants are the same kind of reference, so they read
// through the one participant matcher. A row written before there were ids
// carries only a name, which is why the name travels beside the id.
export function peopleOnFlowTask(task: DeviceFlowTask): DeviceCalendarParticipant[] {
	const identifiers = task.participantIDs ?? [];
	const names = task.participantNames ?? [];
	const written: DeviceCalendarParticipant[] = Array.from(
		{ length: Math.max(identifiers.length, names.length) },
		(_unused, index) => ({ personID: identifiers[index], name: names[index] })
	);
	if (task.ownerID || task.ownerName) written.unshift({ personID: task.ownerID, name: task.ownerName });
	return written;
}

export function flowTaskAsTask(task: DeviceFlowTask, timeZone: string): FlowTaskAsTask {
	const endDay = dayOf(task.endDate);
	const startDay = dayOf(task.startDate) ?? endDay;
	return {
		title: titleOfFlowTask(task),
		note: noteOf(task),
		business: task.business?.trim() || null,
		type: task.type?.trim() || null,
		size: task.size?.trim() || null,
		status: statusOfDevice[task.status?.trim() ?? ''] ?? 'todo',
		startsAt: startDay ? `${startDay}T00:00:00${offsetOn(startDay, timeZone)}` : null,
		endsAt: endDay ? `${endDay}T23:59:00${offsetOn(endDay, timeZone)}` : null,
		isWholeDay: Boolean(startDay && endDay),
		calendar: mirrorsOf(task)
	};
}

// A board writes a day, and a day only becomes an instant in somebody's zone.
// The offset is read for that very day, so a company on summer time gets the
// offset that was in force then.
function offsetOn(day: string, timeZone: string): string {
	const named = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' })
		.formatToParts(new Date(`${day}T12:00:00Z`))
		.find((part) => part.type === 'timeZoneName')?.value;
	const offset = (named ?? '').replace('GMT', '');
	return offset || '+00:00';
}

function noteOf(task: DeviceFlowTask): string | null {
	const goal = task.goal?.trim();
	return [goal && `목표: ${goal}`, task.requestReason?.trim()].filter(Boolean).join('\n') || null;
}

function dayOf(value: string | undefined): string | null {
	const day = (value ?? '').slice(0, 10);
	return /^\d{4}-\d{2}-\d{2}$/.test(day) ? day : null;
}

// A flow task's device id is what makes a second import find the row it wrote
// the first time, so it rides in the same place an event's identities ride.
function mirrorsOf(task: DeviceFlowTask): EventCalendar {
	const mirrors: EventMirror[] = task.id ? [{ source: deviceSource, externalID: task.id }] : [];
	return { mirrors };
}
