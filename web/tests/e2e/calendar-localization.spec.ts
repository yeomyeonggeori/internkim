import { expect, test } from '@playwright/test';

test.describe('calendar localization', () => {
	test('updates embedded calendar labels when language changes', async ({ page }) => {
		let locale = 'ko';
		await page.clock.setFixedTime(new Date('2026-06-08T12:00:00'));
		await page.route('**/admin/api/session', async (route) => {
			await route.fulfill({ json: { email: 'tester@example.com' } });
		});
		await page.route('**/auth/session**', async (route) => {
			await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
		});
		await page.route('**/admin/api/locale', async (route) => {
			if (route.request().method() === 'PUT') {
				const payload = route.request().postDataJSON() as { locale?: string };
				locale = payload.locale === 'en' ? 'en' : 'ko';
			}
			await route.fulfill({ json: { locale } });
		});
		await page.route('**/calendar/api/events?**', async (route) => {
			await route.fulfill({ json: { events: [] } });
		});
		await page.route('**/calendar/api/sync', async (route) => {
			await route.fulfill({
				json: {
					caldavURL: 'https://calendar.example.test/caldav',
					caldavUsername: 'internkim',
					caldavPassword: 'token',
					icsURL: 'https://calendar.example.test/calendar.ics'
				}
			});
		});
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({ json: { connected: false, needsReauth: false } });
		});

		await page.goto('/calendar/');
		await expect(page.getByRole('button', { name: '구독 설정' })).toBeVisible();
		await page.getByRole('button', { name: 'Change language' }).click();
		await page.getByRole('menuitemradio', { name: 'English' }).click();

		await expect(page.getByRole('button', { name: 'Subscription settings' })).toBeVisible();
		await page.getByRole('button', { name: 'Subscription settings' }).click();
		await expect(page.getByText('External calendar account', { exact: true })).toBeVisible();
		await expect(page.getByText('Google Calendar not connected')).toBeVisible();
		await expect(page.getByText('CalDAV/ICS subscription ready')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByText('External calendar account', { exact: true })).toBeHidden();

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: 'Today' })).toBeVisible();
		await expect(calendarFrame.getByRole('button', { name: 'Month' })).toBeVisible();

		await calendarFrame.getByRole('button', { name: 'New', exact: true }).click();
		const popover = calendarFrame.locator('.calendar-draft-popover');
		await expect(popover.getByLabel('Title')).toBeVisible();
		await popover.getByRole('button', { name: /Start date 2026\.06\.08 09:00/ }).click();
		const picker = calendarFrame.locator('.draft-date-time-picker');
		await expect(picker).toHaveAttribute('aria-label', 'Edit start date and time');
		await expect(picker.getByRole('button', { name: 'Previous month' })).toBeVisible();
		await expect(picker.getByRole('button', { name: 'June 17, 2026' })).toBeVisible();
		await expect(picker.getByLabel('Hour')).toBeVisible();
		await expect(picker.getByLabel('Minute')).toBeVisible();
		await expect(picker.getByRole('button', { name: 'Save' })).toBeVisible();
	});
});
