import { expect, type Page, test } from '@playwright/test';

type MockAdminRole = 'admin' | 'operationsAdmin' | 'member';

async function mockAdminPage(page: Page, role: MockAdminRole): Promise<void> {
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
						userID: 'admin-user',
						handle: 'adminuser',
						name: 'Admin User',
						email: 'admin@example.com',
						hireDate: '2026-01-01',
						role: 'admin',
						circles: ['staff']
					}
				],
				availableCircles: [
					{ circleID: 'staff', displayName: 'Staff' },
					{ circleID: 'admin', displayName: 'Admin' },
					{ circleID: 'engineering', displayName: 'Engineering' }
				],
				availableGroups: []
			}
		});
	});
}

test.describe('admin role tabs', () => {
	test('shows every admin tab to full admins', async ({ page }) => {
		await mockAdminPage(page, 'admin');

		await page.goto('/settings/?fleet_id=demo');

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

		await page.goto('/settings/?fleet_id=demo');

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
	});

	test('hides admin navigation from members', async ({ page }) => {
		await mockAdminPage(page, 'member');

		await page.goto('/flow/');

		await expect(page.locator('nav a[href="/admin/"][aria-label="관리"]')).toHaveCount(0);
	});
});
