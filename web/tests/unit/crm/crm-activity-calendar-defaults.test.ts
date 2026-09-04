import { expect, test } from 'bun:test';

import { crmActivityCalendarDefaults } from '../../../src/routes/crm/crm-activity-calendar-defaults';

test('defaults a new calendar-linked activity to end after it starts', () => {
	const defaults = crmActivityCalendarDefaults('2026-06-08T14:15');

	expect(defaults.calendarStart).toBe('2026-06-08T14:15');
	expect(defaults.calendarEnd).toBe('2026-06-08T15:15');
	expect(defaults.calendarEnd > defaults.calendarStart).toBe(true);
});
