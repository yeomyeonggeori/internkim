import { describe, expect, test } from 'bun:test';

import { initialCalendarDate } from '../../../src/routes/calendar/embed/calendar-initial-page-state';

describe('initial calendar date', () => {
	const today = new Date(2026, 6, 29, 9, 30, 0, 0);

	test('opens on today when no date is requested', () => {
		expect(initialCalendarDate(true, new URLSearchParams(), today)).toEqual(today);
	});

	test('opens on the requested date', () => {
		const requested = initialCalendarDate(true, new URLSearchParams('date=2026-03-05'), today);

		expect(requested.getFullYear()).toBe(2026);
		expect(requested.getMonth()).toBe(2);
		expect(requested.getDate()).toBe(5);
	});

	test('opens on today when the requested date is unusable', () => {
		expect(initialCalendarDate(true, new URLSearchParams('date=not-a-date'), today)).toEqual(today);
	});
});
