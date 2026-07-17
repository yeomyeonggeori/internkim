import { describe, expect, test } from 'bun:test';
import { isCalendarEventAccessibleClick } from '../../../src/routes/calendar/embed/calendar-event-accessible-click';

describe('calendar event accessible click', () => {
	test('accepts an untrusted synthesized click regardless of detail', () => {
		expect(isCalendarEventAccessibleClick({ detail: 1, isTrusted: false })).toBe(true);
	});

	test('accepts a trusted keyboard or assistive click with zero detail', () => {
		expect(isCalendarEventAccessibleClick({ detail: 0, isTrusted: true })).toBe(true);
	});

	test('rejects an ordinary trusted pointer click', () => {
		expect(isCalendarEventAccessibleClick({ detail: 1, isTrusted: true })).toBe(false);
	});
});
