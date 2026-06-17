import { test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { expectRenderableCalendarEvent } from './calendar-embed-interaction-assertions';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('renders persisted timed, all-day, and multi-day events across views', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'timed-meeting',
				title: 'Timed Meeting',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'all-day-event',
				title: 'All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'multi-day-event',
				title: 'Multi Day Event',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-12T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '일');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-day-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-day-event.df-event-timed');

		await openCalendarEmbed(page, '주');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-week-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'multi-day-event', '.df-week-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-week-event.df-event-timed');

		await openCalendarEmbed(page, '월');
		await expectRenderableCalendarEvent(page, 'timed-meeting', '.df-month-event.df-event-timed');
		await expectRenderableCalendarEvent(page, 'all-day-event', '.df-month-event.df-event-all-day');
		await expectRenderableCalendarEvent(page, 'multi-day-event', '.df-month-event.df-event-all-day');
	});
});
