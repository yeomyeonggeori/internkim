import { expect, test } from '@playwright/test';
import { expectMultiDayTimedProxy, expectWeekAllDayEventsDoNotOverlap } from './calendar-embed-interaction-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';

test.describe('embedded calendar multi-day proxy interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('shows multi-day timed events as editable day and week all-day proxies', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-multi-day-timed-event',
				title: '주간 멀티',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'week-multi-day-timed-event-2',
				title: '주간 멀티 2',
				startISO: '2026-06-16T13:00:00+09:00',
				endISO: '2026-06-18T14:15:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event', '주간 멀티', '11:45');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event-2', '주간 멀티 2', '13:00');
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);

		await page.locator('.calendar-multi-day-all-day-proxy', { hasText: '주간 멀티' }).first().dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.df-event-detail-panel')).toHaveCount(0);

		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event-2::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);
	});

	test('keeps multi-day timed proxies from overlapping week all-day events', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-all-day-overlap-event',
				title: '주간 종일',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-19T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'week-multi-day-overlap-event',
				title: '주간 시간 다일',
				startISO: '2026-06-16T11:00:00+09:00',
				endISO: '2026-06-18T12:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMultiDayTimedProxy(page, 'week-multi-day-overlap-event', '주간 시간 다일', '11:00');
		await expectWeekAllDayEventsDoNotOverlap(page, 'week-all-day-overlap-event', 'week-multi-day-overlap-event::multi-day-proxy');
	});

	test('does not open a multi-day proxy popover after a moved pointer gesture', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-proxy-drag-guard-event',
				title: '주간 프록시 드래그',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const proxySelector = '.calendar-multi-day-all-day-proxy[data-event-id="week-proxy-drag-guard-event::multi-day-proxy"]';
		await expect(page.locator(proxySelector)).toBeVisible();
		const proxyBox = await page.locator(proxySelector).evaluate((element) => {
			const rectangle = element.getBoundingClientRect();
			return { x: rectangle.left, y: rectangle.top, width: rectangle.width, height: rectangle.height };
		});
		expect(proxyBox.width).toBeGreaterThan(0);
		expect(proxyBox.height).toBeGreaterThan(0);
		const sourceX = proxyBox.x + proxyBox.width / 2;
		const sourceY = proxyBox.y + proxyBox.height / 2;

		await page.mouse.move(sourceX, sourceY);
		await page.mouse.down();
		await page.mouse.move(sourceX + 90, sourceY + 14, { steps: 8 });
		await page.mouse.up();
		await page.locator(proxySelector).dblclick();

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});
});
