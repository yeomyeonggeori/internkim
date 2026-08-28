import { describe, expect, test } from 'bun:test';
import { dayTaskEventSelectorForID } from '../../../src/routes/calendar/embed/calendar-dayflow-dom-adapter';

describe('calendar DayTask DOM adapter', () => {
	test('selects the grid chip, the original event, and synthetic proxy elements for an event ID', () => {
		expect(dayTaskEventSelectorForID('event.1')).toBe(
			'[data-calendar-event-id="event\\.1"], [data-event-id="event\\.1"], [data-event-id^="event\\.1::"]'
		);
	});
});
