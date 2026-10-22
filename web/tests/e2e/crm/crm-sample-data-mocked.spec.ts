import { expect, test } from '@playwright/test';

test('opens the CRM development fixture without the Buzz identity enrollment gate', async ({ page }) => {
	const vaultResponsePromise = page.waitForResponse('**/agent/api/buzz-vault');

	await page.goto('/crm/');

	await expect(page.locator('[data-crm-ready="true"]')).toBeVisible();
	const vaultResponse = await vaultResponsePromise;
	expect(vaultResponse.ok()).toBe(true);
	expect(await vaultResponse.json()).toEqual({ found: true });
	await expect(page.getByRole('dialog', { name: '보안 신원 설정' })).toBeHidden();
});
