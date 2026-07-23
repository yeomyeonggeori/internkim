import { expect, test, type Page } from '@playwright/test';
import { routeCalendarEventCreates, routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';
import { expectTimelinePreviewWithinGrid } from './calendar-embed-interaction-assertions';
import { expectTimelineDraftCount } from './calendar-embed-draft-assertions';
import {
	createDayTimelineRangeByDragAtHour,
	createTimelineRangeByDrag,
	createTimelineSlotByClick,
	finishTimelineRangeDrag,
	openCalendarEmbed,
	startTimelineRangeDrag
} from './calendar-embed-interaction-helpers';

test.describe('embedded calendar timeline drag interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('creates one draft when clicking a timeline slot', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createTimelineSlotByClick(page, '일');
		await expectTimelineDraftCount(page, 1, 2);

		await openCalendarEmbed(page, '주');
		await createTimelineSlotByClick(page, '주');
		await expectTimelineDraftCount(page, 1, 2);
	});

	test('opens an unsaved draft popover when clicking day and week all-day cells', async ({ page }) => {
		const createdEvents = await routeCalendarEventCreates(page);

		await openCalendarEmbed(page, '일');
		await clickAllDayCell(page, '일');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.calendar-draft-popover [aria-label="종일"]')).toBeChecked();
		expect(createdEvents).toHaveLength(0);

		await page.locator('.calendar-draft-popover .draft-popover-cancel').click();
		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);

		await openCalendarEmbed(page, '주');
		await clickAllDayCell(page, '주');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.calendar-draft-popover [aria-label="종일"]')).toBeChecked();
		expect(createdEvents).toHaveLength(0);
	});

	test('creates one draft when dragging a timeline range', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createTimelineRangeByDrag(page, '일');
		await expectTimelineDraftCount(page, 1, 2);

		await openCalendarEmbed(page, '주');
		await createTimelineRangeByDrag(page, '주');
		await expectTimelineDraftCount(page, 1, 2);
	});

	test('creates a day draft from the midnight timeline cell by dragging', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await createDayTimelineRangeByDragAtHour(page, 0, 1);

		await expectTimelineDraftCount(page, 1, 2);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(
			page.locator('.draft-popover-date-time-summary[data-date-time-summary="start"] .draft-popover-date-time-value')
		).toContainText('00:00');
		await expect(
			page.locator('.draft-popover-date-time-summary[data-date-time-summary="end"] .draft-popover-date-time-value')
		).toContainText('01:00');
		await expect(page.locator('.draft-empty-title-event:not(.df-right-panel-event-card)').first().locator('.calendar-event-time')).toHaveText(
			'00:00'
		);
	});

	test('shows timeline range preview with time label and overlap lane while dragging', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'existing-overlap-event',
				title: '겹치는 일정',
				startISO: '2026-06-08T02:00:00+09:00',
				endISO: '2026-06-08T03:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '주');
		await expect(page.locator('[data-event-id="existing-overlap-event"]')).toBeVisible();
		await startTimelineRangeDrag(page, '주');

		const preview = page.locator('.timeline-range-preview');
		await expect(preview).toBeVisible();
		await expect(preview.locator('.timeline-range-preview-time')).toHaveText(/\d{2}:\d{2} - \d{2}:\d{2}/);
		await expectTimelinePreviewWithinGrid(page, '.timeline-range-preview', '.df-week-time-grid-grid');
		await expect(preview).toHaveClass(/calendar-timeline-overlap-adjusted/);
		await expect(page.locator('[data-event-id="existing-overlap-event"]')).toHaveClass(/calendar-timeline-overlap-adjusted/);

		await finishTimelineRangeDrag(page, '주');
	});

	test('clips week timeline range preview inside the visible scroller after scrolling', async ({ page }) => {
		await openCalendarEmbed(page, '주');
		const measurements = await startScrolledCrossDayWeekRangeDrag(page);

		expect(measurements.length).toBeGreaterThan(1);
		for (const measurement of measurements) {
			expect(measurement.topOverflow).toBeLessThanOrEqual(1);
			expect(measurement.bottomOverflow).toBeLessThanOrEqual(1);
		}

		await finishScrolledWeekRangeDrag(page);
	});

	test('keeps day timeline range preview inside the event column while dragging', async ({ page }) => {
		await openCalendarEmbed(page, '일');
		await startTimelineRangeDrag(page, '일');

		const preview = page.locator('.timeline-range-preview');
		await expect(preview).toBeVisible();
		await expectTimelinePreviewWithinGrid(page, '.timeline-range-preview', '.df-day-content-grid-column');

		await finishTimelineRangeDrag(page, '일');
	});
});

async function clickAllDayCell(page: Page, viewLabel: '일' | '주'): Promise<void> {
	const selector = viewLabel === '일' ? '.df-day-content-all-day-lane' : '.df-week-all-day-cell:nth-child(2)';
	await page.waitForSelector(selector, { state: 'attached' });
	await page.evaluate((targetSelector) => {
		const target = document.querySelector<HTMLElement>(targetSelector);
		if (!target) throw new Error(`Missing all-day cell: ${targetSelector}`);
		const rectangle = target.getBoundingClientRect();
		const clientX = rectangle.left + rectangle.width / 2;
		const clientY = rectangle.top + rectangle.height / 2;
		target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
		target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, clientX, clientY }));
	}, selector);
}

async function startScrolledCrossDayWeekRangeDrag(page: Page): Promise<{ topOverflow: number; bottomOverflow: number }[]> {
	return page.evaluate(async () => {
		const scroller = document.querySelector('.df-week-time-grid-scroller');
		if (!(scroller instanceof HTMLElement)) throw new Error('Missing week timeline scroller');
		scroller.scrollTop = 260;
		scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
		await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
		const scrollerRectangle = scroller.getBoundingClientRect();
		const visibleRow = Array.from(document.querySelectorAll('.df-time-grid-row')).find(
			(row): row is HTMLElement =>
				row instanceof HTMLElement && row.getBoundingClientRect().bottom > scrollerRectangle.top + 220
		);
		if (!visibleRow) throw new Error('Missing visible week timeline row');
		const startCell = visibleRow.querySelectorAll('.df-week-time-grid-cell')[3];
		const endCell = visibleRow.querySelectorAll('.df-week-time-grid-cell')[4];
		if (!(startCell instanceof HTMLElement) || !(endCell instanceof HTMLElement)) throw new Error('Missing week cells');
		const startRectangle = startCell.getBoundingClientRect();
		const endRectangle = endCell.getBoundingClientRect();
		const pointerID = 93;
		const startClientX = startRectangle.left + startRectangle.width / 2;
		const endClientX = endRectangle.left + endRectangle.width / 2;
		const startClientY = scrollerRectangle.top + 360;
		const endClientY = startClientY + 144;
		startCell.dispatchEvent(
			new PointerEvent('pointerdown', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: pointerID,
				clientX: startClientX,
				clientY: startClientY
			})
		);
		document.dispatchEvent(
			new PointerEvent('pointermove', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: pointerID,
				clientX: endClientX,
				clientY: endClientY
			})
		);
		await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
		return Array.from(document.querySelectorAll('.timeline-range-preview')).map((preview) => {
			const previewRectangle = preview.getBoundingClientRect();
			return {
				topOverflow: Math.round(scrollerRectangle.top - previewRectangle.top),
				bottomOverflow: Math.round(previewRectangle.bottom - scrollerRectangle.bottom)
			};
		});
	});
}

async function finishScrolledWeekRangeDrag(page: Page): Promise<void> {
	await page.evaluate(() => {
		window.dispatchEvent(
			new PointerEvent('pointerup', {
				bubbles: true,
				cancelable: true,
				button: 0,
				pointerId: 93,
				clientX: 0,
				clientY: 0
			})
		);
	});
}
