import { expect, test, type Page } from '@playwright/test';
import { calendarEmbedPath, signInToCalendar } from './calendar-central-test-utils';

test.describe('embedded calendar month long-press and drag creation', () => {
	test.use({ locale: 'ko-KR' });

	test.beforeEach(async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-10T12:00:00'));
		await routeCalendarHolidays(page);
	});

	test('creates a draft event on double click of an empty month day cell', async ({ page }) => {
		await openDesktopMonthView(page);
		const cell = page.locator('[data-calendar-date="2026-06-10"]');

		await cell.dblclick();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover).toHaveAttribute('aria-label', '새 일정');
		await page.keyboard.press('Escape');
		await expect(popover).toHaveCount(0);
	});

	test('creates a multi-day all-day draft event by dragging across month day cells', async ({ page }) => {
		await openDesktopMonthView(page);
		const startCell = page.locator('[data-calendar-date="2026-06-10"]');
		const endCell = page.locator('[data-calendar-date="2026-06-12"]');
		await startCell.scrollIntoViewIfNeeded();
		await page.waitForTimeout(1000);
		const startBox = await startCell.boundingBox();
		const endBox = await endCell.boundingBox();
		if (!startBox || !endBox) throw new Error('missing month day cell bounding boxes');

		await page.mouse.move(startBox.x + startBox.width / 2, startBox.y + startBox.height / 2);
		await page.mouse.down();
		await page.mouse.move(endBox.x + endBox.width / 2, endBox.y + endBox.height / 2, { steps: 8 });
		await page.mouse.up();

		const popover = page.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('종일')).toBeChecked();
		await page.keyboard.press('Escape');
		await expect(popover).toHaveCount(0);
	});
});

async function routeCalendarHolidays(page: Page): Promise<void> {
	await page.route('**/api/calendar/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], degraded: false } });
	});
}

async function openDesktopMonthView(page: Page): Promise<void> {
	await page.setViewportSize({ width: 1280, height: 900 });
	await signInToCalendar(page);
	await page.goto(`${calendarEmbedPath}?date=2026-06-10`);
	await page.evaluate(() => window.localStorage.setItem('internkim.calendar.view', 'month'));
	await page.reload();
	await expect(page.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
	await expect(page.locator('[data-calendar-date="2026-06-10"]')).toBeVisible();
	await page.waitForTimeout(1200);
}
