import { expect, test } from '@playwright/test';
import { cloneUsersResponse, mockAdminOrgchart } from './admin-orgchart-helpers';
import { initialUsersResponse } from './admin-orgchart-fixtures';

test.describe('admin org chart route fallback', () => {
	test('falls back from hidden device-only section query to users', async ({ page }) => {
		await mockAdminOrgchart(page, {
			deviceManaged: false,
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		for (const section of ['device', 'network']) {
			await page.goto(`/admin/?fleet_id=demo&section=${section}`);

			await expect(page.getByRole('button', { name: '기기' })).toHaveCount(0);
			await expect(page.getByRole('button', { name: '네트워크' })).toHaveCount(0);
			await expect(page.getByRole('heading', { name: '허용된 사용자' })).toBeVisible();
			await expect(page.getByText('WiFi')).toHaveCount(0);
		}
	});
});
