import { expect, test } from '@playwright/test';
import { routeCalendarBackgroundAPI, routeCalendarParticipants } from './calendar-embed-test-utils';

test.describe('calendar localization', () => {
	test('updates calendar labels when language changes', async ({ page }) => {
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
		await routeCalendarBackgroundAPI(page, {
			connected: false,
			needsReauth: false,
			googleOAuthConfigured: false,
			canManageGoogleOAuth: false
		});
		await routeCalendarParticipants(page, []);

		await page.goto('/calendar/');
		await expect(page.getByRole('button', { name: '설정' })).toBeVisible();
		await page.getByRole('button', { name: '언어 변경' }).click();
		await page.getByRole('menuitemradio', { name: 'English' }).click();

		await expect(page.getByRole('button', { name: 'Settings' })).toBeVisible();
		await page.getByRole('button', { name: 'Settings' }).click();
		await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
		await expect(page.getByText('Connected Google Calendar', { exact: true })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Connect' })).toBeHidden();
		await expect(page.getByText('CalDAV/ICS subscription ready')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('heading', { name: 'Settings' })).toBeHidden();

		await expect(page.getByRole('button', { name: 'June 2026' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Month' })).toBeVisible();

		await page.getByRole('button', { name: 'New', exact: true }).click();
		const popover = page.locator('.calendar-draft-popover');
		await expect(popover.getByLabel('Title')).toBeVisible();
		await popover.getByRole('button', { name: /Start date 2026\.06\.08 09:00/ }).click();
		const picker = page.locator('.draft-date-time-picker');
		await expect(picker).toHaveAttribute('aria-label', 'Edit start date and time');
		await expect(picker.getByRole('button', { name: 'Previous month' })).toBeVisible();
		await expect(picker.getByRole('button', { name: 'June 17, 2026' })).toBeVisible();
		await expect(picker.getByLabel('Hour')).toBeVisible();
		await expect(picker.getByLabel('Minute')).toBeVisible();
		await expect(picker.getByRole('button', { name: 'Save' })).toBeVisible();
	});
});
