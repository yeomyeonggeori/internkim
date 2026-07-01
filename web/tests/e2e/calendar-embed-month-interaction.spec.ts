import { expect, test, type Page } from '@playwright/test';
import { routeCalendarEventUpdates, routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import {
	expectMonthEventFullBlockFocused,
	expectMonthOverlayAlignedWithMonthStartRow,
	expectMonthWeekendCellsKeepGridLines
} from './calendar-embed-interaction-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';

test.describe('embedded calendar month interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('focuses the whole month event block when an event is selected', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'focused-all-day-event',
				title: 'Focused All Day',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-09');
		await page.locator('[data-event-id="focused-all-day-event"].calendar-month-direct-event').click();

		await expectMonthEventFullBlockFocused(page, 'focused-all-day-event');
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('selects a month event when activated from the keyboard', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'keyboard-selected-month-event',
				title: 'Keyboard Selected Month Event',
				startISO: '2026-06-09T00:00:00+09:00',
				endISO: '2026-06-10T00:00:00+09:00',
				isAllDay: true
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-09');
		const eventBlock = page.locator('[data-event-id="keyboard-selected-month-event"].calendar-month-direct-event');
		await eventBlock.focus();
		await page.keyboard.press('Enter');

		await expectMonthEventFullBlockFocused(page, 'keyboard-selected-month-event');
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('opens a month event popover only on double click after selecting it', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'double-click-month-event',
				title: 'Double Click Month Event',
				startISO: '2026-06-09T09:00:00+09:00',
				endISO: '2026-06-09T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-09');
		const eventBlock = page.locator('[data-event-id="double-click-month-event"].calendar-month-direct-event');
		await eventBlock.click();

		await expectMonthEventFullBlockFocused(page, 'double-click-month-event');
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await eventBlock.dblclick();

		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.getByLabel('제목')).toHaveValue('Double Click Month Event');
	});

	test('does not open a month event popover after a moved pointer gesture', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 720 });
		await routeCalendarEventUpdates(page);
		await routeCalendarEvents(page, [
			{
				id: 'scroll-touch-month-event',
				title: 'Scroll Touch Month Event',
				startISO: '2026-06-09T09:00:00+09:00',
				endISO: '2026-06-09T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-09');

		await page.evaluate(() => {
			const eventBlock = document.querySelector<HTMLElement>(
				'[data-event-id="scroll-touch-month-event"].calendar-month-direct-event'
			);
			if (!eventBlock) throw new Error('Missing scroll touch month event');
			const rectangle = eventBlock.getBoundingClientRect();
			const pointerID = 42;
			const clientX = rectangle.left + rectangle.width / 2;
			const startClientY = rectangle.top + rectangle.height / 2;
			eventBlock.dispatchEvent(
				new PointerEvent('pointerdown', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX,
					clientY: startClientY
				})
			);
			document.dispatchEvent(
				new PointerEvent('pointermove', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX,
					clientY: startClientY + 72
				})
			);
			document.dispatchEvent(
				new PointerEvent('pointerup', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX,
					clientY: startClientY + 72
				})
			);
			eventBlock.dispatchEvent(
				new MouseEvent('dblclick', {
					bubbles: true,
					cancelable: true,
					button: 0,
					clientX,
					clientY: startClientY + 72
				})
			);
		});

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});

	test('moves the selected month date with arrow keys', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');

		await page.keyboard.press('ArrowRight');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-18');

		await page.keyboard.press('ArrowDown');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-25');

		await page.keyboard.press('ArrowLeft');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-24');

		await page.keyboard.press('ArrowUp');
		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-17');
	});

	test('keeps selected month date aligned with the selected month event', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'aligned-selection-event',
				title: 'Aligned Selection Event',
				startISO: '2026-06-18T19:00:00+09:00',
				endISO: '2026-06-18T21:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await page.locator('[data-event-id="aligned-selection-event"].calendar-month-direct-event').click();

		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-18');
		await expectMonthEventFullBlockFocused(page, 'aligned-selection-event');

		await page.keyboard.press('ArrowLeft');

		await expect.poll(async () => selectedMonthDateKey(page)).toBe('2026-06-17');
		await expect(page.locator('[data-event-id="aligned-selection-event"].internkim-calendar-event-focused')).toHaveCount(0);
	});

	test('keeps month scroll position stable when opening an event popover', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'stable-scroll-event',
				title: 'Stable Scroll Event',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-10T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const scroller = page.locator('.df-month-view-virtual-scroller');
		await expect(scroller).toBeVisible();
		const initialScrollTop = await scroller.evaluate((element) => element.scrollTop);

		await page.locator('[data-event-id="stable-scroll-event"].calendar-month-direct-event').dblclick();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();

		await expect.poll(async () => scroller.evaluate((element) => element.scrollTop)).toBe(initialScrollTop);
	});

	test('clears selected month event when clicking an empty date cell', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'clear-selection-event',
				title: 'Clear Selection Event',
				startISO: '2026-06-10T09:00:00+09:00',
				endISO: '2026-06-10T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '월');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await page.locator('[data-event-id="clear-selection-event"].calendar-month-direct-event').click();
		await expectMonthEventFullBlockFocused(page, 'clear-selection-event');

		await page.locator('.df-month-day-cell[data-date="2026-06-12"]').click({ position: { x: 40, y: 44 } });
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
		await expect(page.locator('[data-event-id="clear-selection-event"].internkim-calendar-event-focused')).toHaveCount(0);
	});

	test('scrolls the month virtual scroller on wheel gestures', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		const scroller = page.locator('.df-month-view-virtual-scroller');
		await expect(scroller).toBeVisible();
		const initialScrollTop = await scroller.evaluate((element) => element.scrollTop);

		await scroller.hover();
		await page.mouse.wheel(0, 640);

		await expect.poll(async () => scroller.evaluate((element) => element.scrollTop)).toBeGreaterThan(initialScrollTop);
	});

	test('shows a month overlay label while scrolling the month view', async ({ page }) => {
		await openCalendarEmbed(page, '월');
		const scroller = page.locator('.df-month-view-virtual-scroller');
		await expect(scroller).toBeVisible();

		await scroller.hover();
		await page.mouse.wheel(0, 640);

		await expect(page.locator('.month-scroll-overlay').first()).toBeVisible();
		await expect(page.locator('.month-scroll-overlay').first()).toHaveText(/\d{4}년 \d{1,2}월/);
		await expectMonthOverlayAlignedWithMonthStartRow(page);
	});

	test('keeps month and week calendar markers legible without hiding grid lines', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-07T03:00:00.000Z'));
		await openCalendarEmbed(page, '월');
		await expectMonthWeekendCellsKeepGridLines(page);
		await expect(page.locator('.df-month-day-cell[data-date="2026-06-07"] .df-month-date-number')).toHaveCSS('color', 'rgb(255, 255, 255)');

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-07');
		await expect(page.locator('.df-week-header > .df-week-day-cell:first-child .df-date-number')).toHaveCSS('color', 'rgb(255, 255, 255)');

		await navigateEmbeddedCalendar(page, '2026-06-16');
		const selectedWeekHeader = page.locator('.df-week-day-cell.calendar-selected-week-date, .df-week-day-header.calendar-selected-week-date');
		await expect(selectedWeekHeader).toBeVisible();
		await expect(selectedWeekHeader).toContainText(/화\s*16/);
		const selectedDateNumber = selectedWeekHeader.locator('.df-date-number, .df-week-date-number').first();
		await expect(selectedDateNumber).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
	});

	test('selects a week header date when clicked', async ({ page }) => {
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-08');

		await page.locator('.df-week-header > .df-week-day-cell').nth(2).click();

		await expect(page.locator('.calendar-stage')).toHaveAttribute('data-calendar-selected-date-key', '2026-06-09');
		const selectedWeekHeader = page.locator('.df-week-day-cell.calendar-selected-week-date, .df-week-day-header.calendar-selected-week-date');
		await expect(selectedWeekHeader).toContainText(/화\s*9/);
	});
});

async function selectedMonthDateKey(page: Page): Promise<string> {
	return page.locator('.calendar-stage').evaluate((element) => element.getAttribute('data-calendar-selected-date-key') ?? '');
}
