import { expect, test, type Page } from '@playwright/test';
import { AttendanceLoadingFixture } from './attendance-loading-fixture';
import { calendarViewStorageKey } from '../../src/routes/calendar/calendar-storage-keys';

const phase = process.env.LOADING_EVIDENCE_PHASE || 'after';
const directory = process.env.LOADING_EVIDENCE_DIR;
async function capture(page: Page, scene: string, width: number): Promise<void> {
	if (directory) await page.screenshot({ animations: 'disabled', path: `${directory}/calendar-${scene}-${width}-${phase}.png` });
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
}
test.use({ locale: 'ko-KR' });
for (const width of [1280, 390, 320]) {
	for (const view of ['month', 'week', 'day']) {
		test(`calendar ${view} pending and loaded at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const fixture = new AttendanceLoadingFixture();
			fixture.calendar.block();
			await fixture.install(page);
			await page.addInitScript(({ key, view }) => localStorage.setItem(key, view), { key: calendarViewStorageKey, view });
			await page.goto('/example-co/calendar?date=2026-10-06');
			await expect(page.locator('.calendar-stage')).toBeVisible();
			await expect.poll(() => fixture.calendar.reads).toBeGreaterThan(0);
			if (phase !== 'before') {
				await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'true');
				await expect(page.locator('[data-calendar-loading-status]')).toBeVisible();
				await expect(page.locator('[data-calendar-loading-cell]')).toHaveCount(0);
			}
			await capture(page, `${view}-pending`, width);
			fixture.calendar.release();
			await expect(page.locator('.calendar-stage').getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
			if (phase !== 'before') await expect(page.locator('[data-calendar-loading-cell]')).toHaveCount(0);
			if (phase !== 'before') await expect(page.locator('[data-calendar-loading-status]')).toHaveCount(0);
			await capture(page, `${view}-loaded`, width);
		});
	}
}

test('calendar refresh keeps known events during delay and failure', async ({ page }) => {
	const fixture = new AttendanceLoadingFixture();
	await fixture.install(page);
	await page.goto('/example-co/calendar?date=2026-10-06');
	await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
	const previousReads = fixture.calendar.reads;
	fixture.calendar.block();
	await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
	await expect.poll(() => fixture.calendar.reads).toBeGreaterThan(previousReads);
	await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
	if (phase !== 'before') await expect(page.locator('[data-calendar-loading-cell]')).toHaveCount(0);
	await capture(page, 'refresh-pending', 1280);
	fixture.calendar.fail = true;
	fixture.calendar.release();
	await expect(page.getByText('샘플 응답을 불러오지 못했습니다')).toBeVisible();
	if (phase !== 'before') await expect(page.getByText('제품 점검', { exact: true }).first()).toBeVisible();
	await capture(page, 'refresh-error', 1280);
});
