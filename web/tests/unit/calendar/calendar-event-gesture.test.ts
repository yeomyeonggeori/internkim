import { describe, expect, test } from 'bun:test';

import {
	hasCalendarEventGestureMoved,
	isCalendarEventDoubleTap,
	type CalendarEventTap
} from '../../../src/routes/calendar/embed/calendar-event-gesture';

describe('calendar event gesture movement', () => {
	test('treats movement beyond 6px as a drag', () => {
		expect(hasCalendarEventGestureMoved({ clientX: 0, clientY: 0 }, { clientX: 6, clientY: 0 })).toBe(false);
		expect(hasCalendarEventGestureMoved({ clientX: 0, clientY: 0 }, { clientX: 7, clientY: 0 })).toBe(true);
	});
});

describe('calendar event double tap', () => {
	test('accepts a normal double tap', () => {
		expect(isCalendarEventDoubleTap(createTap(), createTap({ startTimestamp: 180, endTimestamp: 230 }))).toBe(true);
	});

	test('rejects a second touch that moves 7px', () => {
		expect(
			isCalendarEventDoubleTap(
				createTap(),
				createTap({ startClientX: 10, endClientX: 17, startTimestamp: 180, endTimestamp: 230 })
			)
		).toBe(false);
	});

	test('rejects a long second press', () => {
		expect(
			isCalendarEventDoubleTap(createTap(), createTap({ startTimestamp: 180, endTimestamp: 631 }))
		).toBe(false);
	});

	test('rejects a slow second activation', () => {
		expect(
			isCalendarEventDoubleTap(createTap(), createTap({ startTimestamp: 500, endTimestamp: 550 }))
		).toBe(false);
	});

	test('rejects a second activation for a different event', () => {
		expect(
			isCalendarEventDoubleTap(
				createTap(),
				createTap({ eventID: 'event-2', startTimestamp: 180, endTimestamp: 230 })
			)
		).toBe(false);
	});

	test('uses a 24px double-tap distance limit', () => {
		expect(
			isCalendarEventDoubleTap(
				createTap(),
				createTap({ startClientX: 34, endClientX: 34, startTimestamp: 180, endTimestamp: 230 })
			)
		).toBe(true);
		expect(
			isCalendarEventDoubleTap(
				createTap(),
				createTap({ startClientX: 35, endClientX: 35, startTimestamp: 180, endTimestamp: 230 })
			)
		).toBe(false);
	});
});

type TapOverrides = {
	eventID?: string;
	startClientX?: number;
	startClientY?: number;
	startTimestamp?: number;
	endClientX?: number;
	endClientY?: number;
	endTimestamp?: number;
};

function createTap(overrides: TapOverrides = {}): CalendarEventTap {
	return {
		eventID: overrides.eventID ?? 'event-1',
		start: {
			clientX: overrides.startClientX ?? 10,
			clientY: overrides.startClientY ?? 10,
			timestamp: overrides.startTimestamp ?? 0
		},
		end: {
			clientX: overrides.endClientX ?? 10,
			clientY: overrides.endClientY ?? 10,
			timestamp: overrides.endTimestamp ?? 50
		}
	};
}
