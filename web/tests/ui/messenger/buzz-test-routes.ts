import type { Page } from '@playwright/test';

export async function mockBuzzDisabled(page: Page): Promise<void> {
	await page.route('**/agent/api/buzz-relay-config', async (route) => {
		await route.fulfill({ json: {} });
	});
	await page.route('**/agent/api/buzz-vault', async (route) => {
		await route.fulfill({ json: { found: true } });
	});
}
