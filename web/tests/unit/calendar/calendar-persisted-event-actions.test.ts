import { expect, mock, test } from 'bun:test';

import { dayTaskEventFromCalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-mapping';
import { CalendarProgrammaticUpdateState } from '../../../src/routes/calendar/embed/calendar-programmatic-updates';
import { calendarServerEvent } from './calendar-event-persistence-scenario';

const written: Array<{ targetEventID: string | null; eventID: string }> = [];
const deleted: string[] = [];
const serverEvent = calendarServerEvent('2f1c6b1e-0000-4000-8000-000000000001', 'Created draft');

mock.module('$lib/calendar/supabase-calendar', () => ({
	supabaseCalendarEvents: async () => [],
	supabaseCalendarParticipants: async () => [],
	saveSupabaseCalendarEvent: async (
		payload: { eventID: string },
		targetEventID: string | null
	) => {
		written.push({ targetEventID, eventID: payload.eventID });
		return serverEvent;
	},
	deleteSupabaseCalendarEvent: async (eventID: string) => {
		deleted.push(eventID);
	}
}));

const { createCalendarPersistedEventActions } = await import(
	'../../../src/routes/calendar/embed/calendar-persisted-event-actions'
);

test('addresses later writes and deletes of a created event by the ID the record gave it', async () => {
	const localDraft = dayTaskEventFromCalendarEvent(
		calendarServerEvent('quick-1788331680143-umwpd4', 'Created draft')
	);
	const actions = createCalendarPersistedEventActions(
		{
			getCalendarEvents: () => [localDraft],
			updateCalendarEvent: async () => {},
			setVisibleEvents: () => {},
			text: {
				deleteError: 'Could not delete the event.',
				saveError: 'Could not save the event.'
			}
		},
		new CalendarProgrammaticUpdateState()
	);

	await actions.writeEvent(true, localDraft);
	await actions.writeEvent(false, localDraft);
	await actions.deleteEvent(localDraft.id);

	expect(written).toEqual([
		{ targetEventID: null, eventID: 'quick-1788331680143-umwpd4' },
		{ targetEventID: '2f1c6b1e-0000-4000-8000-000000000001', eventID: '2f1c6b1e-0000-4000-8000-000000000001' }
	]);
	expect(deleted).toEqual(['2f1c6b1e-0000-4000-8000-000000000001']);
});
