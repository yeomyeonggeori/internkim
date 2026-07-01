import { expect, test, type Locator, type Page } from '@playwright/test';
import { routeCalendarShellAPI } from './calendar-route-shell-test-utils';

test.describe('calendar route Google OAuth setup', () => {
	test.beforeEach(async ({ page }) => {
		await routeCalendarShellAPI(page);
	});

	test('shows Google OAuth client upload only for calendar administrators', async ({ page }) => {
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: false,
					needsReauth: false,
					googleOAuthConfigured: false,
					canManageGoogleOAuth: false
				}
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByText('연결된 Google 캘린더')).toHaveCount(0);
		await expect(page.getByText('Google OAuth client.json')).toHaveCount(0);
	});

	test('uploads Google OAuth client JSON and shows connect guidance', async ({ page }) => {
		let isConfigured = false;
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: false,
					needsReauth: false,
					googleOAuthConfigured: isConfigured,
					canManageGoogleOAuth: true
				}
			});
		});
		await page.route('**/calendar/api/google-oauth-client', async (route) => {
			isConfigured = true;
			await route.fulfill({ json: { configured: true, clientID: 'client-1' } });
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);
		await expect(page.getByText('연결된 Google 캘린더', { exact: true })).toBeVisible();
		await expect(page.getByText('Google OAuth client.json')).toBeVisible();
		const googleOAuthClientGuide = page.locator('details', { hasText: 'client.json 만드는 방법' });

		await googleOAuthClientGuide.getByText('client.json 만드는 방법').click();

		await expect(
			googleOAuthClientGuide.getByText('Google Cloud 오른쪽 위 프로필이 회사 관리자 계정인지 확인하세요.')
		).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('상단 프로젝트가 internkim-calendar인지 확인하세요.')).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('CalDAV API를 사용 설정합니다.')).toBeVisible();
		await expect(googleOAuthClientGuide.getByText(/비밀번호가 표시되면 안전한 곳에 따로 저장하세요/)).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('승인된 리디렉션 URI', { exact: true })).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('승인된 JavaScript 원본', { exact: true })).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('https://{본인 서버 URL}/calendar/oauth/google/callback')).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('https://{본인 서버 URL}', { exact: true })).toBeVisible();
		await expect(googleOAuthClientGuide.getByText('example.com')).toHaveCount(0);
		await expect(googleOAuthClientGuide.getByText('Google Calendar API')).toHaveCount(0);
		await expect(googleOAuthClientGuide.getByRole('link')).toHaveCount(3);
		await expect(googleOAuthClientGuide.getByRole('link', { name: '프로젝트 만들기' })).toHaveAttribute(
			'href',
			'https://console.cloud.google.com/projectcreate'
		);
		await expect(googleOAuthClientGuide.getByRole('link', { name: 'CalDAV API 활성화 열기' })).toHaveAttribute(
			'href',
			'https://console.cloud.google.com/marketplace/product/google/caldav.googleapis.com'
		);
		await expect(googleOAuthClientGuide.getByRole('link', { name: 'OAuth 클라이언트 열기' })).toHaveAttribute(
			'href',
			'https://console.cloud.google.com/auth/clients'
		);

		await page.getByLabel('client.json 파일 선택').setInputFiles({
			name: 'client.json',
			mimeType: 'application/json',
			buffer: Buffer.from('{"web":{"client_id":"client-1","client_secret":"secret-1"}}')
		});
		await page.getByRole('button', { name: '업로드' }).click();

		await expect(page.getByText('Google 캘린더를 연결하세요.')).toBeVisible();
		await expect(page.getByRole('link', { name: '연결' })).toBeVisible();
		await expect(page.getByText('secret-1')).toHaveCount(0);
	});

	test('lets calendar administrators replace a configured Google OAuth client JSON', async ({ page }) => {
		let uploadedClientFile = false;
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: false,
					needsReauth: false,
					googleOAuthConfigured: true,
					canManageGoogleOAuth: true
				}
			});
		});
		await page.route('**/calendar/api/google-oauth-client', async (route) => {
			uploadedClientFile = true;
			await route.fulfill({ json: { configured: true, clientID: 'client-2' } });
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);

		await expect(page.getByRole('link', { name: '연결' })).toBeVisible();
		await page.getByText('client.json 교체').click();
		await page.getByLabel('client.json 파일 선택').setInputFiles({
			name: 'client.json',
			mimeType: 'application/json',
			buffer: Buffer.from('{"web":{"client_id":"client-2","client_secret":"secret-2"}}')
		});
		await page.getByRole('button', { name: '업로드' }).click();

		expect(uploadedClientFile).toBe(true);
		await expect(page.getByText('secret-2')).toHaveCount(0);
	});

	test('keeps Google OAuth client file selected when upload fails', async ({ page }) => {
		const longClientFileName =
			'client_secret_2_220769313118-b9pi9vbfv1lvuc6qbb2k8kn9tq2sg3jp.apps.googleusercontent.com.json';
		await page.unroute('**/calendar/api/account-status');
		await page.route('**/calendar/api/account-status', async (route) => {
			await route.fulfill({
				json: {
					connected: false,
					needsReauth: false,
					googleOAuthConfigured: false,
					canManageGoogleOAuth: true
				}
			});
		});
		await page.route('**/calendar/api/google-oauth-client', async (route) => {
			await route.fulfill({
				status: 400,
				body: 'google oauth client file missing client_secret'
			});
		});

		await page.goto('/calendar/');
		await openCalendarSettings(page);
		await page.setViewportSize({ width: 764, height: 900 });
		await expectGoogleOAuthUploadLayoutToFit(page);
		await page.getByLabel('client.json 파일 선택').setInputFiles({
			name: longClientFileName,
			mimeType: 'application/json',
			buffer: Buffer.from('{"web":{"client_id":"client-1"}}')
		});
		await expectGoogleOAuthUploadLayoutToFit(page);
		await page.getByRole('button', { name: '업로드' }).click();

		await expect(page.getByText('google oauth client file missing client_secret')).toBeVisible();
		await expect(page.getByRole('button', { name: '업로드' })).toBeEnabled();
		await expect(page.locator('label[for="google-oauth-client-file"]')).toHaveText(longClientFileName);
		await expectGoogleOAuthUploadLayoutToFit(page);
	});
});

async function openCalendarSettings(page: Page): Promise<void> {
	const calendarFrame = page.frameLocator('iframe');
	await expect(calendarFrame.locator('.calendar-toolbar-title')).toBeVisible();
	const settingsButton = calendarFrame.getByRole('button', { name: '설정' });
	const settingsHeading = page.getByRole('heading', { name: '설정' });
	await expect
		.poll(async () => {
			if ((await settingsHeading.count()) > 0) return true;
			await settingsButton.click();
			await page.waitForTimeout(100);
			return (await settingsHeading.count()) > 0;
		})
		.toBe(true);
	await expect(page.getByRole('heading', { name: '설정' })).toBeVisible();
}

async function expectGoogleOAuthUploadLayoutToFit(page: Page): Promise<void> {
	await expectElementNotToOverflow(page.locator('[data-slot="sheet-content"]'));
	await expectElementNotToOverflow(page.getByRole('group', { name: 'Google OAuth client.json' }));
}

async function expectElementNotToOverflow(locator: Locator): Promise<void> {
	await expect(locator).toBeVisible();
	await expect
		.poll(async () =>
			locator.evaluate((element) => {
				const htmlElement = element as HTMLElement;
				return htmlElement.scrollWidth <= htmlElement.clientWidth;
			})
		)
		.toBe(true);
}
