import { expect, test } from '@playwright/test';

test.describe('calendar localization', () => {
	test('updates embedded calendar labels when language changes', async ({ page }) => {
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
		await page.getByLabel('Language').getByRole('button', { name: 'EN', exact: true }).click();

		await expect(page.getByRole('button', { name: 'Subscription settings' })).toBeVisible();
		await page.getByRole('button', { name: 'Subscription settings' }).click();
		await expect(page.getByText('External calendar account', { exact: true })).toBeVisible();
		await expect(page.getByText('Google Calendar not connected')).toBeVisible();
		await expect(page.getByText('CalDAV/ICS subscription ready')).toBeVisible();

		const calendarFrame = page.frameLocator('iframe');
		await expect(calendarFrame.getByRole('button', { name: 'Today' })).toBeVisible();
		await expect(calendarFrame.getByText('Sun').first()).toBeVisible();
	});
});
