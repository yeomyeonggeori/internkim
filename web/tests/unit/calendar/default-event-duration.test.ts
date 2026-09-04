import { expect, test } from 'bun:test';

import { defaultEventDurationMinutes, defaultEventEndDate } from '../../../src/lib/calendar/default-event-duration';

test('defaults a new event to a one-hour span', () => {
	expect(defaultEventDurationMinutes).toBe(60);
	expect(defaultEventEndDate(new Date(2026, 5, 8, 14, 15, 0, 0))).toEqual(new Date(2026, 5, 8, 15, 15, 0, 0));
});
