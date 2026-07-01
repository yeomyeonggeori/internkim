import type { Page } from '@playwright/test';

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
				caldavURL: '',
				caldavUsername: '',
				caldavPassword: '',
				icsURL: ''
			}
		});
	});
	await page.route('**/calendar/api/account-status', async (route) => {
		await route.fulfill({
			json: {
				connected: false,
				needsReauth: false,
				googleOAuthConfigured: true,
				canManageGoogleOAuth: false
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
	await page.route('**/calendar/api/participants', async (route) => {
		await route.fulfill({ json: { participants: [] } });
	});
	await page.route('**/calendar/api/remote-sync', async (route) => {
		await route.fulfill({ json: { synced: false } });
	});
	await page.route('**/calendar/api/conflicts', async (route) => {
		await route.fulfill({ json: { conflicts: [] } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
}
