import { expect, test } from '@playwright/test';
import {
	dispatchMonthRangePointerDrag,
	draftPopoverMotionStyle,
	dragBetweenCells,
	monthPopoverAnchorGeometry,
	routeCalendarAPI,
	scrollMonthViewBy,
	startDragBetweenCells,
	waitForClientHydration
} from './calendar-draft-popover-test-utils';

test.describe('calendar draft popover anchors', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarAPI(page);
	});

	test('keeps edit popovers attached to their anchor without internal scrolling', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 760 });
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'lower-edit-event',
							title: '하단 편집 일정',
							description: '',
							location: '',
							startISO: '2026-06-17T00:00:00.000Z',
							endISO: '2026-06-18T00:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: true,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="lower-edit-event"]').dblclick();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();

		await expect
			.poll(async () =>
				page.evaluate(() => {
					const eventElement = document.querySelector<HTMLElement>(
						'.calendar-month-direct-event[data-event-id="lower-edit-event"]'
					);
					const titleElement = eventElement?.querySelector<HTMLElement>('.calendar-month-event-title, .calendar-event-title');
					const popover = document.querySelector<HTMLElement>('.calendar-draft-popover');
					const scrollContainer = document.querySelector<HTMLElement>('.draft-popover-scroll');
					if (!titleElement || !popover || !scrollContainer) return false;
					const titleRectangle = titleElement.getBoundingClientRect();
					const popoverRectangle = popover.getBoundingClientRect();
					const arrowTop = Number.parseFloat(window.getComputedStyle(popover, '::before').top);
					const overflowY = window.getComputedStyle(scrollContainer).overflowY;
					const arrowY = popoverRectangle.top + arrowTop + 8;
					const titleCenterY = titleRectangle.top + titleRectangle.height / 2;
					return Math.abs(arrowY - titleCenterY) <= 18 && overflowY !== 'auto' && overflowY !== 'scroll';
				})
			)
			.toBe(true);
	});

	test('moves month edit popovers with their anchor and closes after the anchor leaves the calendar', async ({ page }) => {
		await page.setViewportSize({ width: 1440, height: 560 });
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'scroll-anchor-event',
							title: '스크롤 기준 일정',
							description: '',
							location: '',
							startISO: '2026-06-17T00:00:00.000Z',
							endISO: '2026-06-18T00:00:00.000Z',
							timeZone: 'Asia/Seoul',
							isAllDay: true,
							color: '#1677ff',
							updatedAt: '2026-06-10T11:30:00.000Z'
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await page.locator('.calendar-month-direct-event[data-event-id="scroll-anchor-event"]').dblclick();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		const initialGeometry = await monthPopoverAnchorGeometry(page, 'scroll-anchor-event');

		await scrollMonthViewBy(page, 80);

		await expect
			.poll(async () => {
				const geometry = await monthPopoverAnchorGeometry(page, 'scroll-anchor-event');
				return geometry ? Math.abs(geometry.arrowY - geometry.titleCenterY) : Number.POSITIVE_INFINITY;
			})
			.toBeLessThanOrEqual(18);
		const movedGeometry = await monthPopoverAnchorGeometry(page, 'scroll-anchor-event');
		const movedMotionStyle = await draftPopoverMotionStyle(page);
		expect(movedGeometry?.titleCenterY).toBeLessThan(initialGeometry?.titleCenterY ?? Number.POSITIVE_INFINITY);
		expect(movedGeometry?.arrowY).toBeLessThan(initialGeometry?.arrowY ?? Number.POSITIVE_INFINITY);
		expect(['', 'none']).toContain(movedMotionStyle.transform);

		await scrollMonthViewBy(page, 900);

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('places a dragged month range preview below overlapping all-day events', async ({ page }) => {
		await page.unroute('**/calendar/api/events?**');
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({
				json: {
					events: [
						{
							id: 'existing-all-day',
							title: '기존 종일 일정',
							startISO: '2026-06-10T00:00:00.000Z',
							endISO: '2026-06-13T00:00:00.000Z',
							isAllDay: true
						}
					]
				}
			});
		});
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);
		await expect(page.locator('.calendar-month-direct-event[data-event-id="existing-all-day"]')).toBeVisible();

		await startDragBetweenCells(page, '2026-06-10', '2026-06-12');

		const verticalGap = await page.evaluate(() => {
			const existingEvent = document.querySelector('.calendar-month-direct-event[data-event-id="existing-all-day"]');
			const preview = document.querySelector('.month-range-preview');
			if (!(existingEvent instanceof HTMLElement) || !(preview instanceof HTMLElement)) return Number.NEGATIVE_INFINITY;
			const existingRectangle = existingEvent.getBoundingClientRect();
			const previewRectangle = preview.getBoundingClientRect();
			return Math.round(previewRectangle.top - existingRectangle.bottom);
		});
		expect(verticalGap).toBeGreaterThanOrEqual(2);
		await page.mouse.up();
	});

	test('cancels a dragged month range on pointer cancel without opening a draft', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dispatchMonthRangePointerDrag(page, '2026-06-10', '2026-06-12', 'cancel');

		await expect(page.locator('.month-range-preview')).toHaveCount(0);
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id^="range-"]')).toHaveCount(0);
	});

	test('uses safe timed values when converting a dragged all-day range to timed', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.goto('/calendar/embed');
		await waitForClientHydration(page);

		await dragBetweenCells(page, '2026-06-10', '2026-06-12');

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();

		await popover.getByLabel('종일').uncheck();

		await expect(popover.getByLabel('시작 시간')).toHaveValue('09:00');
		await expect(popover.getByLabel('종료 시간')).toHaveValue('10:00');
	});
});
