import { expect, test } from '@playwright/test';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';

const rowHeightPixels = 18;

test.describe('calendar grid month sizing', () => {
	test('keeps adjacent-date month rows at a fixed height and aligned to their date cells', async ({ page }) => {
		await routeDefaultCalendarAPI(page);
		await routeCalendarEvents(page, [
			{
				id: 'sizing-single-all-day',
				title: 'Single all day',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			...Array.from({ length: 4 }, (_, index) => ({
				id: `sizing-overflow-${index}`,
				title: `Overflow ${index}`,
				startISO: `2026-06-08T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-08T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			...Array.from({ length: 3 }, (_, index) => ({
				id: `sizing-three-timed-${index}`,
				title: `Three timed ${index}`,
				startISO: `2026-06-09T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-09T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			{
				id: 'sizing-all-day-two-timed',
				title: 'All day plus timed',
				startISO: '2026-06-10T00:00:00+09:00',
				endISO: '2026-06-11T00:00:00+09:00',
				isAllDay: true
			},
			...Array.from({ length: 2 }, (_, index) => ({
				id: `sizing-two-timed-${index}`,
				title: `Two timed ${index}`,
				startISO: `2026-06-10T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-10T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			{
				id: 'sizing-week-end-span',
				title: 'Week end span',
				startISO: '2026-06-12T00:00:00+09:00',
				endISO: '2026-06-14T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'sizing-span-start-timed',
				title: 'Span start timed',
				startISO: '2026-06-12T09:00:00+09:00',
				endISO: '2026-06-12T10:00:00+09:00',
				isAllDay: false
			},
			{
				id: 'sizing-span-end-timed',
				title: 'Span end timed',
				startISO: '2026-06-13T09:00:00+09:00',
				endISO: '2026-06-13T10:00:00+09:00',
				isAllDay: false
			}
		]);

		await page.goto('/calendar/embed?date=2026-06-08');
		await page.evaluate(() => {
			window.localStorage.setItem('internkim.calendar.view', 'month');
			window.localStorage.setItem('internkim.calendar.visibleDate', '2026-06-08T12:00:00.000Z');
		});
		await page.reload();
		await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
		await page.evaluate(() => {
			window.postMessage({ type: 'calendar-navigate', dateKey: '2026-06-08' }, window.location.origin);
		});
		await expect(page.locator('[role="grid"]')).toHaveAttribute('aria-label', /2026년 6월/);
		await expect(page.locator('[data-calendar-date="2026-06-08"]')).toBeVisible();

		const measurements = await page.evaluate(() => {
			const eventBox = (eventID: string): DOMRect => {
				const element = document.querySelector<HTMLElement>(`[data-calendar-event-id="${eventID}"]`);
				if (!element) throw new Error(`Missing event ${eventID}`);
				return element.getBoundingClientRect();
			};
			const cellBox = (dateKey: string): DOMRect => {
				const element = document.querySelector<HTMLElement>(`[data-calendar-date="${dateKey}"]`);
				if (!element) throw new Error(`Missing date cell ${dateKey}`);
				return element.getBoundingClientRect();
			};
			const rowBoxes = (eventIDs: string[]): DOMRect[] => eventIDs.map(eventBox);
			const overflowCell = document.querySelector<HTMLElement>('[data-calendar-date="2026-06-08"]');
			if (!overflowCell) throw new Error('Missing overflow date cell');
			const moreButton = overflowCell.querySelector<HTMLElement>('button:not([data-calendar-event-id])');
			if (!moreButton) throw new Error('Missing overflow more button');
			return {
				overflow: [...rowBoxes(['sizing-single-all-day', 'sizing-overflow-0']), moreButton.getBoundingClientRect()],
				threeTimed: rowBoxes(['sizing-three-timed-0', 'sizing-three-timed-1', 'sizing-three-timed-2']),
				allDayTwoTimed: rowBoxes(['sizing-all-day-two-timed', 'sizing-two-timed-0', 'sizing-two-timed-1']),
				cells: ['2026-06-08', '2026-06-09', '2026-06-10'].map(cellBox),
				singleAllDay: eventBox('sizing-single-all-day'),
				singleTimed: eventBox('sizing-overflow-0'),
				weekEndSpan: eventBox('sizing-week-end-span'),
				spanStartTimed: eventBox('sizing-span-start-timed'),
				spanEndTimed: eventBox('sizing-span-end-timed')
			};
		});

		for (const rows of [measurements.overflow, measurements.threeTimed, measurements.allDayTwoTimed]) {
			for (const row of rows) expect(row.height).toBeCloseTo(rowHeightPixels, 0);
			for (let index = 1; index < rows.length; index += 1) expect(rows[index].top - rows[index - 1].bottom).toBeGreaterThanOrEqual(1);
		}
		for (let index = 0; index < measurements.cells.length; index += 1) {
			const cell = measurements.cells[index];
			const rows = [measurements.overflow, measurements.threeTimed, measurements.allDayTwoTimed][index];
			for (const row of rows) expect(row.bottom).toBeLessThanOrEqual(cell.bottom);
		}
		expect(Math.abs(measurements.singleAllDay.left - measurements.singleTimed.left)).toBeLessThanOrEqual(1);
		expect(Math.abs(measurements.weekEndSpan.left - measurements.spanStartTimed.left)).toBeLessThanOrEqual(1);
		expect(Math.abs(measurements.weekEndSpan.right - measurements.spanEndTimed.right)).toBeLessThanOrEqual(1);
	});
});
