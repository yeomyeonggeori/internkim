import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectMonthEventContentAligned,
	expectMonthEventWithinDateCell,
	expectMonthEventsShareBlockStyle,
	expectMonthTimedEventTitleOnly
} from './calendar-embed-interaction-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month layout', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('keeps single-day month events inside their date cell', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'single-all-day-event',
				title: 'Single All Day Event',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-17T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'single-timed-event',
				title: 'Single Timed Event',
				startISO: '2026-06-16T08:00:00+09:00',
				endISO: '2026-06-16T09:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'single-24h-timed-event',
				title: 'Single 24h Timed Event',
				startISO: '2026-06-21T00:00:00+09:00',
				endISO: '2026-06-22T00:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMonthEventWithinDateCell(page, 'single-all-day-event', '2026-06-16');
		await expectMonthEventWithinDateCell(page, 'single-timed-event', '2026-06-16');
		await expectMonthEventWithinDateCell(page, 'single-24h-timed-event', '2026-06-21');
		await expectMonthTimedEventTitleOnly(page, 'single-timed-event', 'Single Timed Event 08:00');
		await expectMonthTimedEventTitleOnly(page, 'single-24h-timed-event', 'Single 24h Timed Event 00:00');
	});

	test('renders month events through a direct event layer instead of DayFlow post-processing', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'direct-layer-event',
				title: 'Direct Layer Event',
				startISO: '2026-06-09T09:00:00+09:00',
				endISO: '2026-06-11T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-09');

		await expect(page.locator('[data-event-id="direct-layer-event"].calendar-month-direct-event')).toHaveCount(1);
		await expect(page.locator('.df-month-week-event-layer-row [data-event-id="direct-layer-event"].df-month-event:visible')).toHaveCount(0);
	});

	test('shows only the start time on multi-day timed month events', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'month-multi-day-timed-event',
				title: 'Month Multi Day Timed',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMonthTimedEventTitleOnly(page, 'month-multi-day-timed-event', 'Month Multi Day Timed 11:45');
	});

	test('orders month events by displayed day span and start time before recent update time', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'later-newer-event',
				title: 'Later Newer',
				startISO: '2026-06-10T10:00:00+09:00',
				endISO: '2026-06-10T11:00:00+09:00',
				isAllDay: false,
				updatedAt: '2026-06-11T00:00:00Z'
			},
			{
				id: 'early-older-event',
				title: 'Early Older',
				startISO: '2026-06-10T08:00:00+09:00',
				endISO: '2026-06-10T09:00:00+09:00',
				isAllDay: false,
				updatedAt: '2026-06-09T00:00:00Z'
			},
			{
				id: 'long-event',
				title: 'Long Event',
				startISO: '2026-06-10T00:00:00+09:00',
				endISO: '2026-06-13T00:00:00+09:00',
				isAllDay: true,
				updatedAt: '2026-06-01T00:00:00Z'
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-10');

		const eventTopByID = await page.evaluate(() => {
			const eventIDs = ['long-event', 'early-older-event', 'later-newer-event'];
			return Object.fromEntries(
				eventIDs.map((eventID) => {
					const element = document.querySelector<HTMLElement>(
						`.calendar-month-direct-event[data-event-id="${CSS.escape(eventID)}"]`
					);
					if (!element) throw new Error(`Missing month event: ${eventID}`);
					return [eventID, Math.round(element.getBoundingClientRect().top)];
				})
			);
		});
		expect(eventTopByID['long-event']).toBeLessThan(eventTopByID['early-older-event']);
		expect(eventTopByID['early-older-event']).toBeLessThan(eventTopByID['later-newer-event']);
	});

	test('uses the same month block style for timed and all-day events', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'style-timed-event',
				title: 'Style Timed',
				startISO: '2026-06-08T09:00:00+09:00',
				endISO: '2026-06-08T10:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'style-all-day-event',
				title: 'Style All Day',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-08');

		await expectMonthEventsShareBlockStyle(page, 'style-timed-event', 'style-all-day-event');
	});

	test('keeps stacked month all-day content aligned with adjusted blocks', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'stacked-three-day-event',
				title: 'Stacked Three Day Event',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-19T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-single-day-event',
				title: 'Stacked Single Day Event',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-17T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'stacked-two-day-event',
				title: 'Stacked Two Day Event',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMonthEventContentAligned(page, 'stacked-three-day-event');
		await expectMonthEventContentAligned(page, 'stacked-single-day-event');
		await expectMonthEventContentAligned(page, 'stacked-two-day-event');
	});

});
