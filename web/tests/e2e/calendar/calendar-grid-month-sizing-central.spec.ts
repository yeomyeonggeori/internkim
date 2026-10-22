import { expect, test } from '@playwright/test';
import {
	calendarEmbedPath,
	cleanupCalendarEvents,
	seedCalendarEvents,
	signInToCalendar
} from './calendar-central-test-utils';

const rowHeightPixels = 18;

test.describe('calendar grid month sizing', () => {
	test.use({ locale: 'ko-KR' });

	test('keeps adjacent-date month rows at a fixed height and aligned to their date cells', async ({ page }) => {
		const eventIDs = await seedCalendarEvents([
			{
				title: 'Single all day',
				startISO: '2026-06-08T00:00:00+09:00',
				endISO: '2026-06-09T00:00:00+09:00',
				isAllDay: true
			},
			...Array.from({ length: 4 }, (_, index) => ({
				title: `Overflow ${index}`,
				startISO: `2026-06-08T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-08T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			...Array.from({ length: 3 }, (_, index) => ({
				title: `Three timed ${index}`,
				startISO: `2026-06-09T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-09T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			{
				title: 'All day plus timed',
				startISO: '2026-06-10T00:00:00+09:00',
				endISO: '2026-06-11T00:00:00+09:00',
				isAllDay: true
			},
			...Array.from({ length: 2 }, (_, index) => ({
				title: `Two timed ${index}`,
				startISO: `2026-06-10T${String(9 + index).padStart(2, '0')}:00:00+09:00`,
				endISO: `2026-06-10T${String(10 + index).padStart(2, '0')}:00:00+09:00`,
				isAllDay: false
			})),
			{
				title: 'Week end span',
				startISO: '2026-06-12T00:00:00+09:00',
				endISO: '2026-06-14T00:00:00+09:00',
				isAllDay: true
			},
			{
				title: 'Span start timed',
				startISO: '2026-06-12T09:00:00+09:00',
				endISO: '2026-06-12T10:00:00+09:00',
				isAllDay: false
			},
			{
				title: 'Span end timed',
				startISO: '2026-06-13T09:00:00+09:00',
				endISO: '2026-06-13T10:00:00+09:00',
				isAllDay: false
			}
		]);
		const [
			singleAllDayID,
			overflow0ID,
			,
			,
			,
			threeTimed0ID,
			threeTimed1ID,
			threeTimed2ID,
			allDayTwoTimedID,
			twoTimed0ID,
			twoTimed1ID,
			weekEndSpanID,
			spanStartTimedID,
			spanEndTimedID
		] = eventIDs;

		try {
			await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
			await signInToCalendar(page);
			await page.goto(`${calendarEmbedPath}?date=2026-06-08`);
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
			await expect(page.locator(`[data-calendar-event-id="${spanEndTimedID}"]`)).toHaveCount(1);

			const measurements = await page.evaluate(
				({ singleAllDayID, overflow0ID, threeTimedIDs, allDayTwoTimedIDs, weekEndSpanID, spanStartTimedID, spanEndTimedID }) => {
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
						overflow: [...rowBoxes([singleAllDayID, overflow0ID]), moreButton.getBoundingClientRect()],
						threeTimed: rowBoxes(threeTimedIDs),
						allDayTwoTimed: rowBoxes(allDayTwoTimedIDs),
						cells: ['2026-06-08', '2026-06-09', '2026-06-10'].map(cellBox),
						singleAllDay: eventBox(singleAllDayID),
						singleTimed: eventBox(overflow0ID),
						weekEndSpan: eventBox(weekEndSpanID),
						spanStartTimed: eventBox(spanStartTimedID),
						spanEndTimed: eventBox(spanEndTimedID)
					};
				},
				{
					singleAllDayID,
					overflow0ID,
					threeTimedIDs: [threeTimed0ID, threeTimed1ID, threeTimed2ID],
					allDayTwoTimedIDs: [allDayTwoTimedID, twoTimed0ID, twoTimed1ID],
					weekEndSpanID,
					spanStartTimedID,
					spanEndTimedID
				}
			);

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
		} finally {
			await cleanupCalendarEvents(eventIDs);
		}
	});
});
