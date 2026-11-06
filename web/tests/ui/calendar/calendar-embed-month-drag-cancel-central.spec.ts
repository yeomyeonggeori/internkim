import { expect, test, type Page } from '@playwright/test';
import { calendarEmbedPath, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar month drag cancellation', () => {
	test.use({ locale: 'ko-KR' });

	test('cancels a month range-selection drag on pointer cancel without opening a new event', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-10T12:00:00'));
		await signInToCalendar(page);
		await page.goto(`${calendarEmbedPath}?date=2026-06-10`);
		await page.evaluate(() => {
			window.localStorage.setItem('internkim.calendar.view', 'month');
		});
		await page.reload();
		await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
		await expect(page.locator('[data-calendar-date="2026-06-10"]')).toBeVisible();
		await expect(page.locator('[data-calendar-date="2026-06-12"]')).toBeVisible();

		await dispatchMonthRangeDragCancel(page, '2026-06-10', '2026-06-12');

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});
});

async function dispatchMonthRangeDragCancel(page: Page, startDateKey: string, endDateKey: string): Promise<void> {
	await page.evaluate(
		({ startDateKey, endDateKey }) => {
			const grid = document.querySelector<HTMLElement>('.calendar-stage [role="grid"]');
			const startCell = document.querySelector<HTMLElement>(`[data-calendar-date="${startDateKey}"]`);
			const endCell = document.querySelector<HTMLElement>(`[data-calendar-date="${endDateKey}"]`);
			if (!grid || !startCell || !endCell) throw new Error('month grid or date cells must be visible before dragging');
			const startRectangle = startCell.getBoundingClientRect();
			const endRectangle = endCell.getBoundingClientRect();
			const pointerID = 51;
			const startClientX = startRectangle.left + startRectangle.width / 2;
			const startClientY = startRectangle.top + startRectangle.height / 2;
			const endClientX = endRectangle.left + endRectangle.width / 2;
			const endClientY = endRectangle.top + endRectangle.height / 2;
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
			grid.dispatchEvent(
				new PointerEvent('pointermove', {
					bubbles: true,
					cancelable: true,
					button: 0,
					pointerId: pointerID,
					clientX: endClientX,
					clientY: endClientY
				})
			);
			grid.dispatchEvent(
				new PointerEvent('pointercancel', {
					bubbles: true,
					cancelable: true,
					pointerId: pointerID,
					clientX: endClientX,
					clientY: endClientY
				})
			);
		},
		{ startDateKey, endDateKey }
	);
}
