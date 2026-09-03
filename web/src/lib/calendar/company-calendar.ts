import { companyTimeZone } from '$lib/company/company-settings';
import { invokeTool } from '$lib/public-api-call';
import { dayOffColor } from './day-off-color';
import { eventReminderLeadOf } from './event-reminder-lead';
import type { CalendarEvent } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';

export type AnsweredParticipant = { personID?: string; name: string; email?: string };

export type AnsweredEvent = {
	eventID: string;
	title: string;
	note: string;
	location: string;
	startsAt: string;
	endsAt: string;
	isWholeDay: boolean;
	notifyMinutesBefore?: number;
	participants: AnsweredParticipant[];
	updatedAt: string;
};

type AnsweredEntry = AnsweredEvent & {
	source: 'event' | 'leave';
	readOnly: boolean;
};

export async function companyCalendarEntries(from: Date, to: Date): Promise<CalendarEvent[]> {
	const [answered, timeZone] = await Promise.all([
		invokeTool<{ events: AnsweredEntry[] }>('event_list', {
			startsAt: from.toISOString(),
			endsAt: to.toISOString()
		}),
		companyTimeZone()
	]);
	return answered.events.map((entry) => calendarEventOfEntry(entry, timeZone));
}

function calendarEventOfEntry(entry: AnsweredEntry, timeZone: string): CalendarEvent {
	const isDayOff = entry.source === 'leave';
	const person = entry.participants[0];
	return {
		id: entry.eventID,
		uid: entry.eventID,
		title: entry.title,
		description: entry.note,
		location: entry.location,
		startISO: entry.startsAt,
		endISO: entry.endsAt,
		timeZone,
		isAllDay: entry.isWholeDay,
		color: isDayOff ? dayOffColor : '',
		participants: entry.participants.map(calendarParticipant),
		createdByEmail: isDayOff ? person?.email ?? '' : '',
		createdByName: isDayOff ? person?.name ?? '' : '',
		reminderMinutesBefore: eventReminderLeadOf(entry.notifyMinutesBefore),
		updatedAt: entry.updatedAt,
		readOnly: entry.readOnly,
		source: entry.source
	};
}

function calendarParticipant(participant: AnsweredParticipant): CalendarParticipant {
	return {
		personID: participant.personID ?? '',
		name: participant.name,
		email: participant.email
	};
}
