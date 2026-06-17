import { describe, expect, test } from 'bun:test';
import { dayFlowEventSelectorForID } from '../../../src/routes/calendar/embed/calendar-dayflow-dom-adapter';

describe('calendar DayFlow DOM adapter', () => {
	test('selects the original event and synthetic proxy elements for an event ID', () => {
		expect(dayFlowEventSelectorForID('event.1')).toBe('[data-event-id="event\\.1"], [data-event-id^="event\\.1::"]');
	});
});
