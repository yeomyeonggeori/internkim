import { expect, test } from '@playwright/test';
import {
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';
import { openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar interactions', () => {
	test.use({ locale: 'ko-KR' });

	test('renders persisted timed, all-day, and multi-day events across views', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Timed Meeting',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:30:00+09:00',
				isAllDay: false
			},
			{
				title: 'All Day Event',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			{
				title: 'Multi Day Event',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-12T00:00:00+09:00',
				isAllDay: true
			}
		]);
		const [timedMeetingID, allDayEventID, multiDayEventID] = eventIDs;

		try {
			await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
			await signInToCalendar(page);

			await openCalendarEmbed(page, '일');
			await expect(page.locator(`[data-calendar-event-id="${allDayEventID}"]:visible`)).toContainText('All Day Event');
			await expect(page.locator(`[data-calendar-event-id="${timedMeetingID}"]:visible`)).toContainText('Timed Meeting');

			await openCalendarEmbed(page, '주');
			await expect(page.locator(`[data-calendar-event-id="${allDayEventID}"]:visible`)).toContainText('All Day Event');
			await expect(page.locator(`[data-calendar-event-id="${multiDayEventID}"]:visible`).first()).toContainText('Multi Day Event');
			await expect(page.locator(`[data-calendar-event-id="${timedMeetingID}"]:visible`)).toContainText('Timed Meeting');

			await openCalendarEmbed(page, '월');
			await expect(page.locator(`[data-calendar-event-id="${timedMeetingID}"]:visible`)).toContainText('Timed Meeting');
			await expect(page.locator(`[data-calendar-event-id="${allDayEventID}"]:visible`)).toContainText('All Day Event');
			await expect(page.locator(`[data-calendar-event-id="${multiDayEventID}"]:visible`).first()).toContainText('Multi Day Event');
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});
});
