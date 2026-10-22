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
	await expect(itemNamed('새 글 채널').getByText('새 글 채널')).toHaveClass(/font-bold/);
	await expect(itemNamed('읽은 채널').getByText('읽은 채널')).not.toHaveClass(/font-bold/);
});

test('the unread count gives way to the more button while the row is hovered or its menu is open', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[
			{ id: 'channel-read', name: '읽은 채널', kind: 'group', myRole: 'member', unreadCount: 0 },
			{ id: 'channel-few', name: '새 글 채널', kind: 'group', myRole: 'member', unreadCount: 6 }
		],
		[]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-read');

	const row = page.locator('[data-sidebar="menu-item"]').filter({ hasText: '새 글 채널' });
	const badge = row.locator('[data-sidebar="menu-badge"]');
	const moreButton = row.getByRole('button', { name: '대화 메뉴' });

	await expect(badge).toHaveCSS('opacity', '1');

	await row.hover();
	await expect(moreButton).toHaveCSS('opacity', '1');
	await expect(badge).toHaveCSS('opacity', '0');

	await moreButton.click();
	await expect(page.getByRole('menuitem', { name: '이 대화 알림 끄기' })).toBeVisible();
	await page.mouse.move(0, 0);
	await expect(badge).toHaveCSS('opacity', '0');

	await page.keyboard.press('Escape');
	await moreButton.blur();
	await expect(moreButton).toHaveCSS('opacity', '0');
	await expect(badge).toHaveCSS('opacity', '1');
});
