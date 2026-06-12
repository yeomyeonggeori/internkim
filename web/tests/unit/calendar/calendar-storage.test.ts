import { describe, expect, test } from 'bun:test';

import { loadSavedCalendarView } from '../../../src/routes/calendar/embed/calendar-storage';

describe('calendar storage', () => {
	test('loads a saved supported calendar view', () => {
		const storage = createStorage({ 'internkim.calendar.view': 'week' });

		expect(loadSavedCalendarView(true, storage)).toBe('week');
	});

	test('falls back to month when saved calendar view is unsupported', () => {
		const storage = createStorage({ 'internkim.calendar.view': 'agenda' });

		expect(loadSavedCalendarView(true, storage)).toBe('month');
	});

});

function createStorage(initialValues: Record<string, string>): Storage {
	const values = new Map(Object.entries(initialValues));
	return {
		get length() {
			return values.size;
		},
		clear: () => values.clear(),
		getItem: (key: string) => values.get(key) ?? null,
		key: (index: number) => Array.from(values.keys())[index] ?? null,
		removeItem: (key: string) => {
			values.delete(key);
		},
		setItem: (key: string, value: string) => {
			values.set(key, value);
		}
	};
}
