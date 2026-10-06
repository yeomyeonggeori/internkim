import { companyTimeZone } from '$lib/company/company-settings';
import { eventReminderLeadOf } from './event-reminder-lead';
import { invokeTool } from '$lib/public-api-call';
import type { CalendarEvent, CalendarEventPayload } from '../../routes/calendar/embed/calendar-event-persistence';
import type { CalendarParticipant } from '../../routes/calendar/embed/calendar-participants';
import { companyCalendarEntries, type AnsweredEvent } from './company-calendar';

type AnsweredPeople = { people: { personID: string; name: string; email: string }[] };

const noReminder = 0;

export async function supabaseCalendarEvents(startDate: Date, endDate: Date, timeZone?: Promise<string>): Promise<CalendarEvent[]> {
	return companyCalendarEntries(startDate, endDate, timeZone);
}

export function calendarEventWritten(payload: CalendarEventPayload): Record<string, unknown> {
	return {
		title: payload.title,
		note: payload.description,
		location: payload.location,
		startsAt: payload.startsAt,
		endsAt: payload.endsAt,
		isWholeDay: payload.isAllDay,
		notifyMinutesBefore: payload.reminderMinutesBefore ?? noReminder,
		participantPersonHints: payload.participants.map((participant) => participant.personID),
		everyoneAttends: payload.isOpenToCompany
	};
}

function versionNamed(expectedUpdatedAt: string | undefined): Record<string, string> {
	return expectedUpdatedAt ? { expectedUpdatedAt } : {};
}

export async function saveSupabaseCalendarEvent(
	payload: CalendarEventPayload,
	targetEventID: string | null
): Promise<CalendarEvent> {
	const written = calendarEventWritten(payload);
	const saved = targetEventID
		? await invokeTool<AnsweredEvent>('event_update', {
				eventHint: targetEventID,
				...versionNamed(payload.expectedUpdatedAt),
				...written
			})
		: await invokeTool<AnsweredEvent>('event_add', written);
	return calendarEventFromAnswer(saved, await companyTimeZone());
}

export async function deleteSupabaseCalendarEvent(
	eventID: string,
	expectedUpdatedAt?: string
): Promise<void> {
	await invokeTool('event_delete', { eventHint: eventID, ...versionNamed(expectedUpdatedAt) });
}

export async function supabaseCalendarParticipants(): Promise<CalendarParticipant[]> {
	const answered = await invokeTool<AnsweredPeople>('person_list', {});
	return answered.people.map((person) => ({
		personID: person.personID,
		name: person.name || person.email.split('@')[0],
		email: person.email || undefined
	}));
}

export function calendarEventFromAnswer(answered: AnsweredEvent, timeZone: string): CalendarEvent {
	return {
		id: answered.eventID,
		uid: answered.eventID,
		title: answered.title,
		description: answered.note,
		location: answered.location,
		startISO: answered.startsAt,
		endISO: answered.endsAt,
		timeZone,
		isAllDay: answered.isWholeDay,
		color: '',
		isOpenToCompany: answered.isOpenToCompany === true,
		participants: answered.participants.map((attendee) => ({
			personID: attendee.personID ?? '',
			name: attendee.name,
			email: attendee.email
		})),
		createdByEmail: '',
		createdByName: '',
		reminderMinutesBefore: eventReminderLeadOf(answered.notifyMinutesBefore),
		updatedAt: answered.updatedAt
	};
}
