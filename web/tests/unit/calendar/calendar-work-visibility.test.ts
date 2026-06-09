import { describe, expect, test } from 'bun:test';

import {
	isCalendarVisibilityMessage,
	loadSavedWorkCalendarVisibility
} from '../../../src/routes/calendar/embed/calendar-work-visibility';

describe('calendar work visibility', () => {
	test('loads work calendar as visible unless storage explicitly stores false', () => {
		expect(loadSavedWorkCalendarVisibility(createStorage(null))).toBe(true);
		expect(loadSavedWorkCalendarVisibility(createStorage('true'))).toBe(true);
		expect(loadSavedWorkCalendarVisibility(createStorage('false'))).toBe(false);
	});

	test('validates broadcast visibility messages', () => {
		expect(isCalendarVisibilityMessage({ type: 'calendar-visibility', work: true })).toBe(true);
		expect(isCalendarVisibilityMessage({ type: 'calendar-visibility', work: false })).toBe(true);
		expect(isCalendarVisibilityMessage({ type: 'calendar-visibility', work: 'false' })).toBe(false);
		expect(isCalendarVisibilityMessage({ type: 'calendar-navigation', work: true })).toBe(false);
		expect(isCalendarVisibilityMessage(null)).toBe(false);
	});
});

function createStorage(storedValue: string | null): Storage {
	return {
		length: storedValue === null ? 0 : 1,
		clear() {},
		getItem() {
			return storedValue;
		},
		key() {
			return storedValue === null ? null : 'internkim.calendar.workVisible';
		},
		removeItem() {},
		setItem() {}
	};
}
