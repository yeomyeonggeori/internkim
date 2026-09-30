import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };

test('a list section folds away under its title and stays folded after a reload', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[
			{ id: 'channel-plaza', name: '샘플광장', kind: 'group', myRole: 'member' },
			{ id: 'dm-sample', name: '박예시', kind: 'dm' }
		],
		[]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-plaza');

	const channelsTitle = page.getByRole('button', { name: '채널', exact: true });
	const channel = page.locator('[data-sidebar="menu-button"]').filter({ hasText: '샘플광장' });
	const directMessage = page.locator('[data-sidebar="menu-button"]').filter({ hasText: '박예시' });

	await expect(channelsTitle).toHaveAttribute('aria-expanded', 'true');
	await expect(channel).toBeVisible();

	await channelsTitle.click();
	await expect(channelsTitle).toHaveAttribute('aria-expanded', 'false');
	await expect(channel).toBeHidden();
	await expect(directMessage).toBeVisible();

	await page.reload();
	await expect(channelsTitle).toHaveAttribute('aria-expanded', 'false');
	await expect(channel).toBeHidden();

	await channelsTitle.click();
	await expect(channel).toBeVisible();
});

test('a section folded in the mobile channel list is folded in the sidebar too', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: 'channel-plaza', name: '샘플광장', kind: 'group', myRole: 'member' }],
		[]
	);
	await page.setViewportSize({ width: 390, height: 800 });
	await page.goto('/messenger?channel=channel-plaza');
	await page.getByRole('button', { name: '채널 목록 열기' }).click();
	const sheet = page.getByRole('dialog');
	await sheet.getByRole('button', { name: '채널', exact: true }).click();
	await expect(sheet.getByText('샘플광장')).toBeHidden();
	await page.keyboard.press('Escape');
	await expect(sheet).toBeHidden();

	await page.setViewportSize({ width: 1280, height: 800 });
	await expect(page.getByRole('button', { name: '채널', exact: true })).toHaveAttribute('aria-expanded', 'false');
	await expect(page.locator('[data-sidebar="menu-button"]').filter({ hasText: '샘플광장' })).toBeHidden();
});
