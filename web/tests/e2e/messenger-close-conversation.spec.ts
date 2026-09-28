import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };

test('choosing the open conversation again closes it', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[
			{ id: 'channel-open', name: '열린 채널', kind: 'group', myRole: 'member', unreadCount: 0 },
			{ id: 'channel-other', name: '다른 채널', kind: 'group', myRole: 'member', unreadCount: 0 }
		],
		[]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-open');

	const buttonNamed = (name: string) =>
		page.locator('[data-sidebar="menu-item"]').filter({ hasText: name }).locator('[data-sidebar="menu-button"]');
	const placeholder = page.getByText('채널이나 대화를 선택하세요');

	await expect(buttonNamed('열린 채널')).toHaveAttribute('data-active', 'true');
	await expect(placeholder).toHaveCount(0);

	await buttonNamed('열린 채널').click();
	await expect(buttonNamed('열린 채널')).not.toHaveAttribute('data-active', 'true');
	await expect(placeholder).toBeVisible();
	await expect(page).not.toHaveURL(/channel=/);

	await buttonNamed('다른 채널').click();
	await expect(buttonNamed('다른 채널')).toHaveAttribute('data-active', 'true');
	await expect(placeholder).toHaveCount(0);
	await expect(page).toHaveURL(/channel=channel-other/);
});
