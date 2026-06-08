import { expect, test } from '@playwright/test';

test.describe('calendar localization', () => {
	test('updates embedded calendar labels when language changes', async ({ page }) => {
		let locale = 'ko';
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
		await page.getByLabel('Language').getByRole('button', { name: 'EN', exact: true }).click();

		await expect(page.getByRole('button', { name: 'Subscription settings' })).toBeVisible();
		await page.getByRole('button', { name: 'Subscription settings' }).click();
		await expect(page.getByText('External calendar account', { exact: true })).toBeVisible();
		await expect(page.getByText('Google Calendar not connected')).toBeVisible();
		await expect(page.getByText('CalDAV/ICS subscription ready')).toBeVisible();

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: 'Today' })).toBeVisible();
		await expect(calendarFrame.getByRole('button', { name: 'Month' })).toBeVisible();
	});
});
