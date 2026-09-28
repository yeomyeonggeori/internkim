import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };

test('the conversation list shows how many messages each conversation has unread', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[
			{ id: 'channel-read', name: '읽은 채널', kind: 'group', myRole: 'member', unreadCount: 0 },
			{ id: 'channel-few', name: '새 글 채널', kind: 'group', myRole: 'member', unreadCount: 3 },
			{ id: 'channel-many', name: '많은 채널', kind: 'group', myRole: 'member', unreadCount: 150 }
		],
		[]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-read');

	const itemNamed = (name: string) => page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });

	await expect(itemNamed('새 글 채널').locator('[data-sidebar="menu-badge"]')).toHaveText(/^읽지 않은 메시지\s*3$/);
	await expect(itemNamed('많은 채널').locator('[data-sidebar="menu-badge"]')).toHaveText(/^읽지 않은 메시지\s*99\+$/);
	await expect(itemNamed('읽은 채널').locator('[data-sidebar="menu-badge"]')).toHaveCount(0);
	await expect(itemNamed('새 글 채널').getByText('새 글 채널')).toHaveClass(/font-semibold/);
	await expect(itemNamed('읽은 채널').getByText('읽은 채널')).not.toHaveClass(/font-semibold/);
});
