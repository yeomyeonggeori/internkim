import { expect, test } from 'bun:test';

import { searchDateLabel } from '../../../src/routes/calendar/embed/calendar-search';

test('formats calendar search result dates with numeric date and weekday code', () => {
	expect(searchDateLabel(new Date(2026, 6, 15, 9, 30))).toBe('2026.07.15 WED');
});
