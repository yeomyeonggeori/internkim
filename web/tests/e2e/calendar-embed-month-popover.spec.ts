import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month popovers', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('points split month event popovers at the first title segment', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'split-anchor-event',
				title: 'Split Anchor Event',
				startISO: '2026-06-06T09:00:00+09:00',
				endISO: '2026-06-09T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-08');
		await expect(page.locator('[data-event-id="split-anchor-event"].calendar-month-direct-event')).toHaveCount(2);
		const splitEvent = await page.evaluate((eventID) => {
			const segments = Array.from(document.querySelectorAll<HTMLElement>(`[data-event-id="${CSS.escape(eventID)}"].df-month-event`))
				.filter((element) => {
					const rectangle = element.getBoundingClientRect();
					return rectangle.width > 0 && rectangle.height > 0;
				})
				.sort((firstElement, secondElement) => {
					const firstRectangle = firstElement.getBoundingClientRect();
					const secondRectangle = secondElement.getBoundingClientRect();
					return firstRectangle.top - secondRectangle.top || firstRectangle.left - secondRectangle.left;
				});
			const firstSegment = segments[0];
			const clickedSegment = segments[1];
			if (!firstSegment || !clickedSegment) throw new Error(`Missing split visible segments for ${eventID}`);
			const clickedRectangle = clickedSegment.getBoundingClientRect();
			const titleElement = firstSegment.querySelector<HTMLElement>('.calendar-month-event-title, .calendar-event-title');
			const firstRectangle = firstSegment.getBoundingClientRect();
			const titleRectangle = titleElement?.getBoundingClientRect() ?? firstRectangle;
			return {
				clickX: clickedRectangle.left + Math.min(48, clickedRectangle.width / 2),
				clickY: clickedRectangle.top + clickedRectangle.height / 2,
				firstTitleCenterY: titleRectangle.top + titleRectangle.height / 2,
				title: {
					left: titleRectangle.left,
					right: titleRectangle.right,
					top: titleRectangle.top,
					bottom: titleRectangle.bottom
				},
				firstSegment: {
					left: firstRectangle.left,
					right: firstRectangle.right
				}
			};
		}, 'split-anchor-event');

		await page.mouse.dblclick(splitEvent.clickX, splitEvent.clickY);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate((expectedCenterY) => {
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!popover) return Number.POSITIVE_INFINITY;
					const popoverRectangle = popover.getBoundingClientRect();
					const arrowTop = Number.parseFloat(window.getComputedStyle(popover, '::before').top);
					return Math.abs(popoverRectangle.top + arrowTop + 8 - expectedCenterY);
				}, splitEvent.firstTitleCenterY)
			)
			.toBeLessThanOrEqual(18);

		await expect
			.poll(async () =>
				page.evaluate((firstSegment) => {
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!popover) return false;
					const popoverRectangle = popover.getBoundingClientRect();
					return popover.classList.contains('popover-arrow-left')
						? popoverRectangle.left >= firstSegment.right + 8
						: popoverRectangle.right <= firstSegment.left - 8;
				}, splitEvent.firstSegment)
			)
			.toBe(true);

		await expect
			.poll(async () =>
				page.evaluate((titleRectangle) => {
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!popover) return true;
					const popoverRectangle = popover.getBoundingClientRect();
					return !(
						popoverRectangle.left < titleRectangle.right &&
						popoverRectangle.right > titleRectangle.left &&
						popoverRectangle.top < titleRectangle.bottom &&
						popoverRectangle.bottom > titleRectangle.top
					);
				}, splitEvent.title)
			)
			.toBe(true);
	});

	test('opens right-edge month event popovers on the left side of the clicked block', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'right-edge-popover-event',
				title: 'Right Edge Popover Event',
				startISO: '2026-06-20T09:00:00+09:00',
				endISO: '2026-06-20T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-20');
		await page.locator('[data-event-id="right-edge-popover-event"].calendar-month-direct-event').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => {
					const eventElement = document.querySelector<HTMLElement>(
						'[data-event-id="right-edge-popover-event"].calendar-month-direct-event'
					);
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!eventElement || !popover) return false;
					const eventRectangle = eventElement.getBoundingClientRect();
					const popoverRectangle = popover.getBoundingClientRect();
					return popover.classList.contains('popover-arrow-right') && popoverRectangle.right <= eventRectangle.left - 8;
				})
			)
			.toBe(true);
	});

	test('keeps the more popover open and points edit popovers at the more row', async ({ page }) => {
		await routeCalendarEvents(
			page,
			Array.from({ length: 8 }, (_, index) => ({
				id: `more-edit-event-${index}`,
				title: `More Edit Event ${index}`,
				startISO: `2026-06-17T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-17T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false,
				updatedAt: `2026-06-${String(10 + index).padStart(2, '0')}T00:00:00Z`
			}))
		);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		await page.locator('.calendar-month-more-button[data-date-key="2026-06-17"]').click();
		const morePopover = page.locator('.calendar-month-more-popover');
		await expect(morePopover).toBeVisible();
		const hiddenEventRow = morePopover.locator('.calendar-month-more-popover-event').first();
		const hiddenEventRowDisplay = await hiddenEventRow.evaluate((element) => {
			const title = element.querySelector('.calendar-month-more-popover-event-title')?.textContent?.trim() ?? '';
			const date = element.querySelector('.calendar-month-more-popover-event-date')?.textContent?.trim() ?? '';
			const markerStyle = window.getComputedStyle(element, '::before');
			return {
				date,
				markerBackground: markerStyle.backgroundColor,
				markerWidth: markerStyle.width,
				title
			};
		});
		expect(hiddenEventRowDisplay.title).toMatch(/^More Edit Event \d+$/);
		expect(hiddenEventRowDisplay.title).not.toMatch(/\d{2}:\d{2}/);
		expect(hiddenEventRowDisplay.date).toMatch(/^6\. 17\. \d{2}:00$/);
		expect(hiddenEventRowDisplay.markerWidth).toBe('3px');
		expect(hiddenEventRowDisplay.markerBackground).toBe('rgb(59, 130, 246)');
		const hiddenEventGeometry = await hiddenEventRow.evaluate((element) => {
			const rectangle = element.getBoundingClientRect();
			return {
				centerY: rectangle.top + rectangle.height / 2,
				left: rectangle.left,
				right: rectangle.right
			};
		});

		await hiddenEventRow.dblclick();

		await expect(morePopover).toBeVisible();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect
			.poll(async () =>
				page.evaluate((expectedCenterY) => {
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!popover) return Number.POSITIVE_INFINITY;
					const popoverRectangle = popover.getBoundingClientRect();
					const arrowTop = Number.parseFloat(window.getComputedStyle(popover, '::before').top);
					return Math.abs(popoverRectangle.top + arrowTop + 8 - expectedCenterY);
				}, hiddenEventGeometry.centerY)
			)
			.toBeLessThanOrEqual(18);
		await expect
			.poll(async () =>
				page.evaluate((rowGeometry) => {
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					if (!popover) return false;
					const popoverRectangle = popover.getBoundingClientRect();
					return popover.classList.contains('popover-arrow-right')
						? popoverRectangle.right <= rowGeometry.left - 8
						: popoverRectangle.left >= rowGeometry.right + 8;
				}, hiddenEventGeometry)
			)
			.toBe(true);
	});
});
