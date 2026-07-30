import { describe, expect, test } from 'bun:test';
import { createCalendarModelEvent as createEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import {
	draftPopoverChanges,
	draftPopoverStartDateTimeChanges,
	draftPopoverStateFromEvent,
	hasDraftPopoverEventChanges
} from '../../../src/routes/calendar/embed/calendar-draft-popover-state';

test('calendar draft popover carries participants through changes', () => {
	const event = createEvent({
		id: 'participant-draft',
		title: 'Participant draft',
		description: 'Bring agenda',
		start: new Date(2026, 5, 18, 12),
		end: new Date(2026, 5, 18, 13),
		allDay: false,
		calendarId: 'internkim',
		meta: {
			location: 'Studio',
			participants: [{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }]
		}
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	expect(popover.participants).toEqual([{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }]);
	expect(draftPopoverChanges(popover).meta.participants).toEqual([
		{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' }
	]);
	expect(hasDraftPopoverEventChanges({ ...popover, participants: [] }, event)).toBe(true);
});

test('keeps the existing duration when a timed draft popover start moves after the current end', () => {
	const event = createEvent({
		id: 'duration-draft',
		title: 'Duration draft',
		start: new Date(2026, 5, 18, 10),
		end: new Date(2026, 5, 18, 12),
		allDay: false,
		calendarId: 'internkim'
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	const changes = draftPopoverStartDateTimeChanges(popover, '2026-06-18', '14:00');

	expect(changes).toMatchObject({
		dateKey: '2026-06-18',
		startTime: '14:00',
		endDateKey: '2026-06-18',
		endTime: '16:00'
	});
});

