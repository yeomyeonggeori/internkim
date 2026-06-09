import { describe, expect, test } from 'bun:test';

import { isCalendarNavigationMessage } from '../../../src/routes/calendar/calendar-navigation-message';

describe('calendar navigation messages', () => {
	test('accepts a valid calendar navigation date key', () => {
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-07-15' })).toBe(true);
	});

	test('rejects malformed calendar navigation date keys', () => {
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-7-15' })).toBe(false);
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-02-31' })).toBe(false);
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: 'not-a-date' })).toBe(false);
	});
});
