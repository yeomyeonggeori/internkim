import { describe, expect, test } from 'bun:test';

import {
	hasCalendarEventGestureMoved,
	isCalendarEventTap
} from '../../../src/routes/calendar/embed/calendar-event-gesture';

describe('calendar event gesture movement', () => {
	test('treats movement beyond 6px as a drag', () => {
		expect(hasCalendarEventGestureMoved({ clientX: 0, clientY: 0 }, { clientX: 6, clientY: 0 })).toBe(false);
		expect(hasCalendarEventGestureMoved({ clientX: 0, clientY: 0 }, { clientX: 7, clientY: 0 })).toBe(true);
	});
});

describe('calendar event tap', () => {
	test('accepts a stationary short touch', () => {
		expect(
			isCalendarEventTap({
				eventID: 'event-1',
				start: { clientX: 10, clientY: 10, timestamp: 0 },
				end: { clientX: 10, clientY: 10, timestamp: 100 }
			})
		).toBe(true);
	});

	test('rejects a moved or long touch', () => {
		expect(
			isCalendarEventTap({
				eventID: 'event-1',
				start: { clientX: 10, clientY: 10, timestamp: 0 },
				end: { clientX: 17, clientY: 10, timestamp: 100 }
			})
		).toBe(false);
		expect(
			isCalendarEventTap({
				eventID: 'event-1',
				start: { clientX: 10, clientY: 10, timestamp: 0 },
				end: { clientX: 10, clientY: 10, timestamp: 451 }
			})
		).toBe(false);
	});
});
