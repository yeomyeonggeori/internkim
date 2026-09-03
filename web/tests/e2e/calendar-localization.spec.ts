import { expect, test } from '@playwright/test';
import { signInToCalendar } from './calendar-central-test-utils';

test.describe('calendar localization', () => {
	test.use({ locale: 'ko-KR' });

	test('updates embedded calendar labels when language changes', async ({ page }) => {
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.route('**/api/calendar/holidays?**', async (route) => {
			await route.fulfill({ json: { holidays: [], degraded: false } });
		});
		await page.route('**/api/calendar/subscription', async (route) => {
			if (route.request().method() === 'POST') {
				await route.fulfill({ json: { address: 'https://calendar.example.test/calendar.ics' } });
				return;
			}
			await route.fulfill({ json: { registered: true } });
		});

		await signInToCalendar(page);
		let calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: '설정' })).toBeVisible();

		await page.getByRole('button', { name: '언어 변경' }).click();
		await page.getByRole('menuitemradio', { name: 'English' }).click();

		calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: 'Settings' })).toBeVisible();
		await calendarFrame.getByRole('button', { name: 'Settings' }).click();
		await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
		await expect(page.getByText('Subscription URL ready')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('heading', { name: 'Settings' })).toBeHidden();

		await expect(calendarFrame.locator('.calendar-toolbar-title')).toHaveText('June 2026');
		await expect(calendarFrame.getByRole('tab', { name: 'Month', exact: true })).toBeVisible();

		await calendarFrame.getByRole('tab', { name: 'Month', exact: true }).click();
		await expect(calendarFrame.locator('.calendar-stage')).toHaveClass(/calendar-stage-month/);
		await calendarFrame.locator('[data-calendar-date="2026-06-08"]').dblclick();
		const popover = calendarFrame.locator('.calendar-draft-popover');
		await expect(popover).toBeVisible();
		await expect(popover.getByLabel('Title')).toBeVisible();
		await popover.getByRole('button', { name: 'Start date' }).click();
		await expect(calendarFrame.locator('[data-calendar-grid]')).toBeVisible();
		await expect(popover.getByLabel('Start time')).toBeVisible();
	});
});
