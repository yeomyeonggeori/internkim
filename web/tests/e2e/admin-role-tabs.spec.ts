import { expect, type Page, test } from '@playwright/test';

type MockAdminRole = 'admin' | 'operationsAdmin' | 'member';

type MockAdminPageOptions = {
	holidayCountriesStatus?: number;
};

async function mockAdminPage(page: Page, role: MockAdminRole, options: MockAdminPageOptions = {}): Promise<void> {
	await page.route('**/admin/api/session', async (route) => {
		if (role === 'member') {
			await route.fulfill({ status: 403, body: 'admin access required' });
			return;
		}
		await route.fulfill({
			json: {
				email: 'operator@example.com',
				claimedAdminEmail: 'owner@example.com',
				isAdmin: role === 'admin',
				role,
				isClaimed: true,
				bootstrapStatus: 'claimed',
				deviceManaged: true
			}
		});
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: role === 'member' ? 'member@example.com' : 'operator@example.com' } });
	});
	await page.route('**/admin/api/users?includePolicy=true', async (route) => {
		await route.fulfill({
			json: {
				records: [
					{
						memberID: 'admin-user',
						handle: 'adminuser',
						name: 'Admin User',
						email: 'admin@example.com',
						hireDate: '2026-01-01',
						role: 'admin',
						circles: ['member']
					}
				],
				availableCircles: [
					{ circleID: 'member', displayName: 'Member' },
					{ circleID: 'admin', displayName: 'Admin' },
					{ circleID: 'engineering', displayName: 'Engineering' }
				],
				availableGroups: []
			}
		});
	});
	await page.route('**/admin/api/workspace-settings', async (route) => {
		await route.fulfill({ json: { countryCode: 'KR', timeZone: 'Asia/Seoul', language: 'ko', callingCode: '82' } });
	});
	await page.route('**/admin/api/holiday-countries', async (route) => {
		if (options.holidayCountriesStatus) {
			await route.fulfill({ status: options.holidayCountriesStatus, body: 'fetch supported holiday countries: nager unavailable' });
			return;
		}
		await route.fulfill({ json: { countries: [{ countryCode: 'KR', name: '대한민국' }] } });
	});
	await page.route('**/admin/api/attendance-locations', async (route) => {
		await route.fulfill({ json: { locations: [{ id: 'office', name: '사무실', color: '#2563eb', isDefault: true }] } });
	});
	await page.route('**/admin/api/calendar-holidays/status', async (route) => {
		await route.fulfill({
			json: {
				status: 'degraded',
				countryCode: 'KR',
				provider: 'nager',
				years: [{ year: 2026, status: 'degraded', cacheCount: 18, lastError: 'nager API status 503: unavailable' }]
			}
		});
	});
}

test.describe('admin role tabs', () => {
	test('keeps the configured country visible when the country list fails to load', async ({ page }) => {
		await mockAdminPage(page, 'operationsAdmin', { holidayCountriesStatus: 502 });

		await page.goto('/?fleet_id=demo');

		const main = page.locator('main');
		await main.getByRole('tab', { name: '일반', exact: true }).click();
		await expect(main.getByText('대한민국 (KR)', { exact: true })).toBeVisible();
		await expect(main.getByText('국가 목록을 불러오지 못했습니다. 현재 설정된 국가는 계속 표시됩니다.', { exact: true })).toBeVisible();
		await expect(main.getByText('fetch supported holiday countries: nager unavailable', { exact: true })).toHaveCount(0);
		await expect(main.getByText('설정을 불러오지 못했습니다.', { exact: true })).toHaveCount(0);
	});

	test('matches the country dropdown width to its trigger', async ({ page }) => {
		await mockAdminPage(page, 'operationsAdmin');

		await page.goto('/?fleet_id=demo');

		const main = page.locator('main');
		await main.getByRole('tab', { name: '일반', exact: true }).click();
		const countryTrigger = main.getByRole('combobox', { name: '회사 국가', exact: true });
		await countryTrigger.click();
		const countryContent = page.locator('[data-slot="popover-content"]');
		await expect(countryContent).toBeVisible();
		await expect
			.poll(async () => {
				const triggerBox = await countryTrigger.boundingBox();
				const contentBox = await countryContent.boundingBox();
				if (!triggerBox || !contentBox) return Number.POSITIVE_INFINITY;
				return Math.abs(contentBox.width - triggerBox.width);
			})
			.toBeLessThanOrEqual(1);
	});

	test('shows every admin tab to full admins', async ({ page }) => {
		await mockAdminPage(page, 'admin');

		await page.goto('/?fleet_id=demo');

		const main = page.locator('main');
		await expect(main.getByRole('tab', { name: '기기' })).toBeVisible();
		await expect(main.getByRole('tab', { name: '사용자' })).toBeVisible();
		await expect(main.getByRole('link', { name: '조직도' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '조직도' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '인증 정보' })).toBeVisible();
		await expect(main.getByRole('tab', { name: '백업' })).toBeVisible();
		await expect(main.getByRole('tab', { name: '봇' })).toBeVisible();
		await expect(main.getByRole('tab', { name: '일반', exact: true })).toBeVisible();
		await expect(main.getByRole('tab', { name: '근무 설정', exact: true })).toBeVisible();
		await expect(main.getByRole('tab', { name: '휴가 설정', exact: true })).toBeVisible();
		await expect(main.getByRole('tab', { name: '네트워크' })).toBeVisible();
		await main.getByRole('tab', { name: '사용자' }).click();
		const adminRow = main.getByRole('row').filter({ hasText: 'admin@example.com' });
		await expect(adminRow.getByText('관리자', { exact: true })).toBeVisible();
		await expect(adminRow.getByRole('button', { name: '일반으로 변경' })).toBeVisible();
	});

	test('limits operations admins to user and settings tabs', async ({ page }) => {
		await mockAdminPage(page, 'operationsAdmin');

		await page.goto('/?fleet_id=demo');

		const main = page.locator('main');
		await expect(main.getByRole('tab', { name: '사용자' })).toBeVisible();
		await expect(main.getByRole('link', { name: '조직도' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '조직도' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '일반', exact: true })).toBeVisible();
		await expect(main.getByRole('tab', { name: '근무 설정', exact: true })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '휴가 설정', exact: true })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '기기' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '인증 정보' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '백업' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '봇' })).toHaveCount(0);
		await expect(main.getByRole('tab', { name: '네트워크' })).toHaveCount(0);
		await main.getByRole('tab', { name: '사용자' }).click();
		const adminRow = main.getByRole('row').filter({ hasText: 'admin@example.com' });
		await expect(adminRow.getByText('관리자', { exact: true })).toBeVisible();
		await expect(adminRow.getByRole('button', { name: '일반으로 변경' })).toHaveCount(0);
		await expect(adminRow.getByRole('button', { name: '저장' })).toBeDisabled();
		await expect(adminRow.getByRole('button', { name: '비밀번호 리셋' })).toBeDisabled();
		await expect(adminRow.getByRole('button', { name: '삭제' })).toBeDisabled();
		await expect(main.getByRole('button', { name: '삭제 Admin' })).toHaveCount(0);
		await expect(main.getByRole('button', { name: '삭제 Engineering' })).toBeVisible();
		await main.getByRole('tab', { name: '일반', exact: true }).click();
		await expect(main.getByRole('button', { name: '지금 다시 시도' })).toBeVisible();
		await expect(main.getByText('nager API status 503: unavailable')).toBeVisible();
	});

	test('hides admin navigation from members', async ({ page }) => {
		await mockAdminPage(page, 'member');

		await page.goto('/flow/');

		await expect(page.locator('nav a[href="/admin/"][aria-label="관리"]')).toHaveCount(0);
	});
});
