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


test('calendar draft popover opens on the reminder an event already carries', () => {
	const reminded = createEvent({
		id: 'reminded-event',
		title: '주간 회의',
		start: new Date(2026, 5, 18, 12),
		end: new Date(2026, 5, 18, 13),
		allDay: false,
		calendarId: 'internkim',
		meta: { reminderMinutesBefore: 30 }
	});
	const unreminded = createEvent({
		id: 'unreminded-event',
		title: '주간 회의',
		start: new Date(2026, 5, 18, 12),
		end: new Date(2026, 5, 18, 13),
		allDay: false,
		calendarId: 'internkim'
	});

	expect(draftPopoverStateFromEvent(reminded, 'edit', null, null).reminderMinutesBefore).toBe(30);
	expect(draftPopoverStateFromEvent(unreminded, 'edit', null, null).reminderMinutesBefore).toBeNull();
});

test('calendar draft popover reports a reminder that was set, changed, or taken away', () => {
	const event = createEvent({
		id: 'reminder-change',
		title: '주간 회의',
		start: new Date(2026, 5, 18, 12),
		end: new Date(2026, 5, 18, 13),
		allDay: false,
		calendarId: 'internkim',
		meta: { reminderMinutesBefore: 30 }
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	expect(hasDraftPopoverEventChanges(popover, event)).toBe(false);
	expect(draftPopoverChanges(popover).meta.reminderMinutesBefore).toBe(30);
	expect(hasDraftPopoverEventChanges({ ...popover, reminderMinutesBefore: 60 }, event)).toBe(true);
	expect(hasDraftPopoverEventChanges({ ...popover, reminderMinutesBefore: null }, event)).toBe(true);
	expect(draftPopoverChanges({ ...popover, reminderMinutesBefore: null }).meta.reminderMinutesBefore).toBeNull();
});
