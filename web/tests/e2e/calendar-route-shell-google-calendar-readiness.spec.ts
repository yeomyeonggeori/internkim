import { expect, test } from '@playwright/test';
import { openCalendarSettings, routeCalendarShellAPI, routeWritableGoogleCalendars } from './calendar-route-shell-test-utils';

test.describe('calendar route Google Calendar readiness', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
		await routeWritableGoogleCalendars(page, { setSelectedCalendarID: () => {} });
	});

	test('shows selected Google Calendar sync readiness', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company@example.com',
					selectedCalendarName: '회사 일정',
					selectedCalendarAccessRole: 'writer',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: true,
					calendarSyncReady: true,
					calendarReadinessStatus: 'sync_ready',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('연결됨: calendar-admin@example.com')).toBeVisible();
		await expect(page.getByText('사용 중인 캘린더: 회사 일정')).toBeVisible();
		await expect(page.getByText('동기화 가능')).toBeVisible();
		const switchAccountLink = page.getByRole('link', { name: '계정 변경' });
		await expect(switchAccountLink).toBeVisible();
		await expect(switchAccountLink).toHaveAttribute('target', '_blank');
		await expect(switchAccountLink).toHaveAttribute('href', /switchAccount=true/);
		const titleBox = await page.getByText('연결된 Google 캘린더', { exact: true }).boundingBox();
		const switchAccountBox = await switchAccountLink.boundingBox();
		expect(titleBox).not.toBeNull();
		expect(switchAccountBox).not.toBeNull();
		if (!titleBox || !switchAccountBox) return;
		expect(switchAccountBox.width).toBeLessThan(140);
		expect(Math.abs(switchAccountBox.y - titleBox.y)).toBeLessThan(16);
	});

	test('shows selected Google Calendar initial sync progress', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company@example.com',
					selectedCalendarName: '회사 일정',
					selectedCalendarAccessRole: 'writer',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: false,
					calendarSyncReady: false,
					calendarReadinessStatus: 'initial_sync_pending',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('사용 중인 캘린더: 회사 일정')).toBeVisible();
		await expect(page.getByText('초기 동기화 중')).toBeVisible();
		await expect(page.getByText('동기화 가능')).toHaveCount(0);
	});

	test('shows selected Google Calendar initial export progress separately', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company@example.com',
					selectedCalendarName: '회사 일정',
					selectedCalendarAccessRole: 'writer',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: false,
					calendarSyncReady: false,
					calendarReadinessStatus: 'initial_export_pending',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('사용 중인 캘린더: 회사 일정')).toBeVisible();
		await expect(page.getByText('초기 내보내기 중')).toBeVisible();
		await expect(page.getByText('초기 동기화 중')).toHaveCount(0);
	});

	test('keeps long selected Google Calendar status contained on mobile', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company-long-calendar@example.com',
					selectedCalendarName: '회사 전체 일정 및 외부 협력사 공유 캘린더 긴 이름',
					selectedCalendarAccessRole: 'writer',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: false,
					calendarSyncReady: false,
					calendarReadinessStatus: 'initial_sync_pending',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		const sheet = page.locator('[data-slot="sheet-content"]');
		const selectedCalendar = page.getByText(/사용 중인 캘린더:/);
		const readiness = page.getByText('초기 동기화 중');
		await expect(selectedCalendar).toBeVisible();
		await expect(readiness).toBeVisible();
		const sheetBox = await sheet.boundingBox();
		const selectedCalendarBox = await selectedCalendar.boundingBox();
		const readinessBox = await readiness.boundingBox();
		expect(sheetBox).not.toBeNull();
		expect(selectedCalendarBox).not.toBeNull();
		expect(readinessBox).not.toBeNull();
		if (!sheetBox || !selectedCalendarBox || !readinessBox) return;
		expect(selectedCalendarBox.x).toBeGreaterThanOrEqual(sheetBox.x);
		expect(selectedCalendarBox.x + selectedCalendarBox.width).toBeLessThanOrEqual(sheetBox.x + sheetBox.width + 1);
		expect(readinessBox.x).toBeGreaterThanOrEqual(sheetBox.x);
		expect(readinessBox.x + readinessBox.width).toBeLessThanOrEqual(sheetBox.x + sheetBox.width + 1);
	});

	test('shows reconnect state when Google calendar selection requires reauth', async ({ page }) => {
		let didFailCalendarSelection = false;
		await page.unroute('**/calendar/api/account-status');
		await page.unroute('**/calendar/api/google-calendars**');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: '',
					selectedCalendarName: '',
					selectedCalendarAccessRole: '',
					needsReauth: didFailCalendarSelection,
					needsCalendarSelection: !didFailCalendarSelection,
					initialSyncCompleted: false,
					calendarSyncReady: false,
					calendarReadinessStatus: didFailCalendarSelection ? 'reauth_required' : 'calendar_selection_required',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});
		await page.route('**/calendar/api/google-calendars**', async (route) => {
			if (route.request().method() === 'GET') {
				await route.fulfill({
					json: {
						accountEmail: 'calendar-admin@example.com',
						calendars: [
							{
								calendarID: 'company@example.com',
								summary: '회사 일정',
								accessRole: 'writer',
								timeZone: 'Asia/Seoul',
								primary: false,
								backgroundColor: '#0ea5e9',
								canWrite: true,
								canSelect: true
							}
						]
					}
				});
				return;
			}
			didFailCalendarSelection = true;
			await route.fulfill({
				status: 502,
				body: 'google calendar readiness status 401: Unauthorized'
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);
		await page.getByLabel('사용할 캘린더').selectOption('company@example.com');
		await page.getByRole('button', { name: '캘린더 저장' }).click();

		await expect.poll(() => didFailCalendarSelection).toBe(true);
		await expect(page.getByText('재연결 필요').first()).toBeVisible();
		await expect(page.getByRole('link', { name: '다시 연결' })).toBeVisible();
		await expect(page.getByLabel('사용할 캘린더')).toHaveCount(0);
	});

	test('shows write permission requirement for read-only selected calendars', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company@example.com',
					selectedCalendarName: '회사 일정',
					selectedCalendarAccessRole: 'reader',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: true,
					calendarSyncReady: false,
					calendarReadinessStatus: 'write_permission_required',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('사용 중인 캘린더: 회사 일정')).toBeVisible();
		await expect(page.getByText('쓰기 권한 필요')).toBeVisible();
	});

	test('shows calendar inaccessible status for unavailable selected calendars', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: true,
					accountEmail: 'calendar-admin@example.com',
					selectedCalendarID: 'company@example.com',
					selectedCalendarName: '회사 일정',
					selectedCalendarAccessRole: 'writer',
					needsReauth: false,
					needsCalendarSelection: false,
					initialSyncCompleted: true,
					calendarSyncReady: false,
					calendarReadinessStatus: 'calendar_inaccessible',
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('사용 중인 캘린더: 회사 일정')).toBeVisible();
		await expect(page.getByText('캘린더 접근 불가')).toBeVisible();
	});
});
