import { expect, test } from '@playwright/test';
import {
	openCalendarSettings,
	routeCalendarShellAPI,
	routeConnectedGoogleCalendarAccount,
	routeWritableGoogleCalendars
} from './calendar-route-shell-test-utils';

test.describe('calendar route Google Calendar selection', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('shows writable Google calendars and saves the selected calendar', async ({ page }) => {
		let selectedCalendarID = '';
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID,
					selectedCalendarName: selectedCalendarID ? '가족' : '',
					selectedCalendarAccessRole: selectedCalendarID ? 'owner' : '',
					needsReauth: false,
					needsCalendarSelection: selectedCalendarID === '',
					initialSyncCompleted: false,
					calendarSyncReady: false,
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});
		await routeWritableGoogleCalendars(page, {
			setSelectedCalendarID: (calendarID) => {
				selectedCalendarID = calendarID;
			}
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('캘린더 선택 필요')).toBeVisible();
		await expect(page.getByLabel('사용할 캘린더')).toBeVisible();
		await expect(page.getByLabel('사용할 캘린더')).toContainText('가족');
		await page.getByLabel('사용할 캘린더').selectOption('family@example.com');
		await page.getByRole('button', { name: '캘린더 저장' }).click();

		await expect(page.getByText('사용 중인 캘린더: 가족')).toBeVisible();
		expect(selectedCalendarID).toBe('family@example.com');
	});

	test('shows read-only Google calendars as disabled choices', async ({ page }) => {
		await routeConnectedGoogleCalendarAccount(page);
		await routeWritableGoogleCalendars(page, {
			setSelectedCalendarID: () => {},
			calendars: [
				{
					calendarID: 'readonly@example.com',
					summary: '읽기 전용',
					accessRole: 'reader',
					primary: false,
					backgroundColor: '#94a3b8',
					canWrite: false,
					canSelect: false,
					selectionDisabledReason: 'write_permission_required'
				},
				{
					calendarID: 'family@example.com',
					summary: '가족',
					accessRole: 'owner',
					primary: false,
					backgroundColor: '#0ea5e9'
				}
			]
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		const calendarSelect = page.getByLabel('사용할 캘린더');
		await expect(calendarSelect).toBeVisible();
		await expect(calendarSelect).toHaveValue('family@example.com');
		await expect(calendarSelect.locator('option[value="readonly@example.com"]')).toBeDisabled();
		await expect(calendarSelect).toContainText('읽기 전용 - 쓰기 권한 필요');
	});

	test('keeps the calendar selector visible while saving a selected calendar', async ({ page }) => {
		let selectedCalendarID = '';
		let shouldDelayAccountStatusReload = false;
		let didStartDelayedAccountStatusReload = false;
		let releaseAccountStatusReload: () => void = () => {};
		const accountStatusReloadGate = new Promise<void>((resolve) => {
			releaseAccountStatusReload = resolve;
		});
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			if (shouldDelayAccountStatusReload) {
				didStartDelayedAccountStatusReload = true;
				await accountStatusReloadGate;
			}
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID,
					selectedCalendarName: selectedCalendarID ? '가족' : '',
					selectedCalendarAccessRole: selectedCalendarID ? 'owner' : '',
					needsReauth: false,
					needsCalendarSelection: selectedCalendarID === '',
					initialSyncCompleted: false,
					calendarSyncReady: false,
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});
		await routeWritableGoogleCalendars(page, {
			setSelectedCalendarID: (calendarID) => {
				selectedCalendarID = calendarID;
			},
			calendars: [
				{
					calendarID: 'family@example.com',
					summary: '가족',
					accessRole: 'owner',
					primary: false,
					backgroundColor: '#0ea5e9'
				}
			]
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);
		await expect(page.getByLabel('사용할 캘린더')).toBeVisible();

		shouldDelayAccountStatusReload = true;
		await page.getByRole('button', { name: '캘린더 저장' }).click();
		await expect.poll(() => didStartDelayedAccountStatusReload).toBe(true);

		await expect(page.getByLabel('사용할 캘린더')).toBeVisible();
		await expect(page.getByRole('button', { name: '저장 중' })).toBeVisible();

		releaseAccountStatusReload();
		await expect(page.getByText('사용 중인 캘린더: 가족')).toBeVisible();
	});

});
