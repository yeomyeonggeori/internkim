import { expect, test } from '@playwright/test';
import {
	routeCalendarShellAPI,
	routeConnectedGoogleCalendarAccount
} from './calendar-route-shell-test-utils';

test.describe('calendar route Google OAuth return', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('opens settings with a connection notice after Google OAuth returns', async ({ page }) => {
		await routeConnectedGoogleCalendarAccount(page);
		await page.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({
				json: {
					accountEmail: 'calendar-admin@example.com',
					calendars: []
				}
			});
		});

		await page.goto('/calendar/?googleOAuth=connected');

		await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
		await expect(page.getByText('Google 캘린더가 연결됐습니다.')).toBeVisible();
		await expect(page).toHaveURL(/\/calendar\/?$/);
	});

	test('opens settings with a connection failure notice after Google OAuth fails', async ({ page }) => {
		await routeConnectedGoogleCalendarAccount(page);
		await page.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({
				json: {
					accountEmail: 'calendar-admin@example.com',
					calendars: []
				}
			});
		});

		await page.goto('/calendar/?googleOAuth=failed');

		await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
		const failureNotice = page.getByText('Google 캘린더 연결에 실패했습니다.');
		await expect(failureNotice).toBeVisible();
		await expect(failureNotice).toHaveClass(/text-destructive/);
		await expect(page).toHaveURL(/\/calendar\/?$/);
	});

	test('notifies the existing calendar tab when Google OAuth returns in a new tab', async ({ page, context }) => {
		await routeConnectedGoogleCalendarAccount(page);
		await page.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({ json: { accountEmail: 'calendar-admin@example.com', calendars: [] } });
		});
		await page.goto('/calendar/');
		await expect(page.getByRole('heading', { name: '설정' })).toHaveCount(0);

		const callbackPage = await context.newPage();
		await routeCalendarShellAPI(callbackPage);
		await routeConnectedGoogleCalendarAccount(callbackPage);
		await callbackPage.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({ json: { accountEmail: 'calendar-admin@example.com', calendars: [] } });
		});
		await callbackPage.goto('/calendar/?googleOAuth=connected');

		await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
		await expect(page.getByText('Google 캘린더가 연결됐습니다.')).toBeVisible();
		await callbackPage.close();
	});

	test('notifies the existing calendar tab when Google OAuth fails in a new tab', async ({ page, context }) => {
		await routeConnectedGoogleCalendarAccount(page);
		await page.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({ json: { accountEmail: 'calendar-admin@example.com', calendars: [] } });
		});
		await page.goto('/calendar/');
		await expect(page.getByRole('heading', { name: '설정' })).toHaveCount(0);

		const callbackPage = await context.newPage();
		await routeCalendarShellAPI(callbackPage);
		await routeConnectedGoogleCalendarAccount(callbackPage);
		await callbackPage.route('**/calendar/api/google-calendars', async (route) => {
			await route.fulfill({ json: { accountEmail: 'calendar-admin@example.com', calendars: [] } });
		});
		await callbackPage.goto('/calendar/?googleOAuth=failed');

		await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
		await expect(page.getByText('Google 캘린더 연결에 실패했습니다.')).toBeVisible();
		await callbackPage.close();
	});

});
