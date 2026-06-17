import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month overflow', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('collapses overflowing same-day month events behind a more button without crossing the date cell boundary', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'overflow-long-event',
				title: 'Overflow Long',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-20T00:00:00+09:00',
				isAllDay: true,
				updatedAt: '2026-06-01T00:00:00Z'
			},
			...Array.from({ length: 8 }, (_, index) => ({
				id: `overflow-short-${index}`,
				title: `Overflow Short ${index}`,
				startISO: '2026-06-17T09:00:00+09:00',
				endISO: '2026-06-17T10:00:00+09:00',
				isAllDay: false,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		const moreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]');
		await expect(moreButton).toBeVisible();
		await expect(moreButton).toContainText(/\+\d+ 더보기/);
		await expect(moreButton).toHaveAttribute('aria-label', /숨겨진 일정 \d+개 보기/);
		await expect(page.locator('.df-month-more-events')).toBeHidden();
		await expect(page.getByText(/\+\d+ more/)).toBeHidden();

		const overflowMeasurements = await page.evaluate(() => {
			const cell = document.querySelector<HTMLElement>('.df-month-day-cell[data-date="2026-06-17"]');
			if (!cell) throw new Error('Missing 2026-06-17 date cell');
			const cellRectangle = cell.getBoundingClientRect();
			const eventRectangles = Array.from(
				document.querySelectorAll<HTMLElement>('.calendar-month-direct-event[data-event-id^="overflow-"]')
			).map((element) => element.getBoundingClientRect());
			return {
				cellBottom: Math.round(cellRectangle.bottom),
				visibleEventCount: eventRectangles.length,
				maxEventBottom: Math.max(...eventRectangles.map((rectangle) => Math.round(rectangle.bottom)))
			};
		});
		expect(overflowMeasurements.visibleEventCount).toBeLessThan(9);
		expect(overflowMeasurements.maxEventBottom).toBeLessThanOrEqual(overflowMeasurements.cellBottom - 18);

		await moreButton.click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		await expect(morePopover.locator('.calendar-month-more-popover-event')).toHaveCount(9 - overflowMeasurements.visibleEventCount);
	});

	test('shows more buttons on every covered date when hidden multi-day events overflow', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 10 }, (_, index) => ({
				id: `overflow-multi-day-${index}`,
				title: `Overflow Multi Day ${index}`,
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-21T00:00:00+09:00',
				isAllDay: true,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		const coveredDateMoreButton = page.locator('.calendar-month-more-button[data-date-key="2026-06-18"]');
		await expect(coveredDateMoreButton).toBeVisible();
		await expect(coveredDateMoreButton).toContainText(/\+\d+ 더보기/);

		await coveredDateMoreButton.click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		await expect(morePopover.locator('.calendar-month-more-popover-event').first()).toContainText('Overflow Multi Day');
	});
});
