import { describe, expect, test } from 'bun:test';

import {
	isCalendarEventsChangedMessage,
	isCalendarNavigationMessage,
	isCalendarOpenSettingsMessage,
	isCalendarRefreshMessage,
	isCalendarViewMessage,
	isCalendarVisibleDateMessage
} from '../../../src/routes/calendar/calendar-navigation-message';

describe('calendar navigation messages', () => {
	test('accepts a valid calendar navigation date key', () => {
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-07-15' })).toBe(true);
	});

	test('rejects malformed calendar navigation date keys', () => {
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-7-15' })).toBe(false);
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: '2026-02-31' })).toBe(false);
		expect(isCalendarNavigationMessage({ type: 'calendar-navigate', dateKey: 'not-a-date' })).toBe(false);
	});

	test('accepts visible date messages with a valid date key', () => {
		expect(isCalendarVisibleDateMessage({ type: 'calendar-visible-date', dateKey: '2026-07-15' })).toBe(true);
	});

	test('rejects malformed visible date messages', () => {
		expect(isCalendarVisibleDateMessage({ type: 'calendar-visible-date', dateKey: '2026-7-15' })).toBe(false);
		expect(isCalendarVisibleDateMessage({ type: 'calendar-visible-date', dateKey: '2026-02-31' })).toBe(false);
		expect(isCalendarVisibleDateMessage({ type: 'calendar-visible-date' })).toBe(false);
	});

	test('accepts supported calendar view messages', () => {
		expect(isCalendarViewMessage({ type: 'calendar-view', view: 'month' })).toBe(true);
		expect(isCalendarViewMessage({ type: 'calendar-view', view: 'week' })).toBe(true);
		expect(isCalendarViewMessage({ type: 'calendar-view', view: 'day' })).toBe(true);
	});

	test('rejects unsupported calendar view messages', () => {
		expect(isCalendarViewMessage({ type: 'calendar-view', view: 'agenda' })).toBe(false);
		expect(isCalendarViewMessage({ type: 'calendar-view' })).toBe(false);
		expect(isCalendarViewMessage({ type: 'calendar-view', view: 1 })).toBe(false);
	});

	test('accepts events changed messages without extra payload requirements', () => {
		expect(isCalendarEventsChangedMessage({ type: 'calendar-events-changed' })).toBe(true);
	});

	test('accepts calendar shell action messages without extra payload requirements', () => {
		expect(isCalendarOpenSettingsMessage({ type: 'calendar-open-settings' })).toBe(true);
		expect(isCalendarRefreshMessage({ type: 'calendar-refresh' })).toBe(true);
		expect(isCalendarOpenSettingsMessage({ type: 'calendar-refresh' })).toBe(false);
		expect(isCalendarRefreshMessage({ type: 'calendar-open-settings' })).toBe(false);
	});
});
