import { type Page } from '@playwright/test';

export async function routeCalendarShellAPI(page: Page): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: 'tester@example.com' } });
	});
	await page.route('**/calendar/api/sync', async (route) => {
		await route.fulfill({
			json: {
				caldavURL: 'https://calendar.example.test/calendar/dav/team/calendars/internkim/',
				caldavUsername: 'internkim',
				caldavPassword: 'a-subscription-token',
				icsURL: 'https://calendar.example.test/calendar/ics/a-subscription-token.ics'
			}
		});
	});
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					{
						id: 'event-2026-06-15',
						title: 'Shell event',
						startISO: '2026-06-15T09:00:00.000Z',
						endISO: '2026-06-15T10:00:00.000Z',
						isAllDay: false
					}
				]
			}
		});
	});
	await page.route('**/calendar/api/holidays?**', async (route) => {
		await route.fulfill({ json: { holidays: [], source: 'holiday_api' } });
	});
	await page.route('**/calendar/api/participants', async (route) => {
		await route.fulfill({ json: { participants: [] } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
}
