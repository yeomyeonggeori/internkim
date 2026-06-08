import { expect, test } from 'bun:test';

import {
	timelineRangeDraftEventParams,
	timelineSingleDraftEventParams
} from '../../../src/routes/calendar/embed/calendar-draft-event-params';

test('creates a one-hour timeline draft from a single slot', () => {
	const startDate = new Date(2026, 5, 8, 14, 15, 30, 500);

	const params = timelineSingleDraftEventParams(startDate);

	expect(params.id.startsWith('timeline-')).toBe(true);
	expect(params.start).toEqual(new Date(2026, 5, 8, 14, 15, 0, 0));
	expect(params.end).toEqual(new Date(2026, 5, 8, 15, 15, 0, 0));
	expect(params.allDay).toBe(false);
	expect(params.calendarId).toBe('internkim');
});

test('orders timeline range dates and keeps at least thirty minutes', () => {
	const laterDate = new Date(2026, 5, 8, 15, 0, 0, 0);
	const earlierDate = new Date(2026, 5, 8, 14, 45, 0, 0);

	const params = timelineRangeDraftEventParams(laterDate, earlierDate);

	expect(params.id.startsWith('timeline-')).toBe(true);
	expect(params.start).toEqual(new Date(2026, 5, 8, 14, 45, 0, 0));
	expect(params.end).toEqual(new Date(2026, 5, 8, 15, 15, 0, 0));
	expect(params.allDay).toBe(false);
	expect(params.calendarId).toBe('internkim');
});
