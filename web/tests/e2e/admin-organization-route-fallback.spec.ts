import { expect, test } from '@playwright/test';
import { cloneUsersResponse, mockAdminOrganization } from './admin-organization-helpers';
import { initialUsersResponse } from './admin-organization-fixtures';

test.describe('admin org chart route fallback', () => {
	test('falls back from hidden device-only section query to users', async ({ page }) => {
		await mockAdminOrganization(page, {
			deviceManaged: false,
			getUsersResponse: () => cloneUsersResponse(initialUsersResponse)
		});

		for (const section of ['device', 'network']) {
			await page.goto(`/?fleet_id=demo&section=${section}`);

			await expect(page.getByRole('tab', { name: '기기' })).toHaveCount(0);
			await expect(page.getByRole('tab', { name: '네트워크' })).toHaveCount(0);
			await expect(page.getByRole('tab', { name: '사용자' })).toHaveAttribute('aria-selected', 'true');
			await expect(page.getByText('WiFi')).toHaveCount(0);
		}
	});
});
