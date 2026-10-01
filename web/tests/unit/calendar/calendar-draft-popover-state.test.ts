import { describe, expect, test } from 'bun:test';
import { createCalendarModelEvent as createEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import {
	draftPopoverAllDayChanges,
	draftPopoverChanges,
	draftPopoverStartDateTimeChanges,
	draftPopoverStateFromEvent,
	hasDraftPopoverEventChanges,
	isDraftPopoverValid,
	type DraftPopoverState
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
			participants: [{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com' }]
		}
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	expect(popover.participants).toEqual([{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com' }]);
	expect(draftPopoverChanges(popover).meta.participants).toEqual([
		{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com' }
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

test('unchecking all day on a zeroed draft gives it the same start and end time the calendar offers a click-created timed draft', () => {
	const draggedAllDayEvent = createEvent({
		id: 'all-day-draft',
		title: '',
		start: new Date(2026, 5, 18),
		end: new Date(2026, 5, 18),
		allDay: true,
		calendarId: 'internkim'
	});
	const popover = draftPopoverStateFromEvent(draggedAllDayEvent, 'create', null, null);
	expect(popover.startTime).toBe('00:00');
	expect(popover.endTime).toBe('00:00');

	const changes = draftPopoverAllDayChanges(popover, false);

	expect(changes).toEqual({ allDay: false, startTime: '09:00', endTime: '10:00' });

	const timedPopover: DraftPopoverState = { ...popover, ...changes };
	expect(isDraftPopoverValid(timedPopover)).toBe(true);
});

test('unchecking all day keeps a draft its own times when they are not the zeroed default', () => {
	const event = createEvent({
		id: 'converted-draft',
		title: 'Retreat',
		start: new Date(2026, 5, 18, 13),
		end: new Date(2026, 5, 18, 15),
		allDay: false,
		calendarId: 'internkim'
	});
	const popover: DraftPopoverState = { ...draftPopoverStateFromEvent(event, 'edit', null, null), allDay: true };

	expect(draftPopoverAllDayChanges(popover, false)).toEqual({ allDay: false });
});

test('checking all day back on leaves the times untouched', () => {
	const event = createEvent({
		id: 'toggled-on-draft',
		title: 'Workshop',
		start: new Date(2026, 5, 18, 9),
		end: new Date(2026, 5, 18, 10),
		allDay: false,
		calendarId: 'internkim'
	});
	const popover = draftPopoverStateFromEvent(event, 'edit', null, null);

	expect(draftPopoverAllDayChanges(popover, true)).toEqual({ allDay: true });
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
