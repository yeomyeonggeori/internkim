import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };

const conversations = [
	{ id: 'channel-open', name: '열린 채널', kind: 'group' as const, myRole: 'member' },
	{ id: 'channel-held', name: '누를 채널', kind: 'group' as const, myRole: 'member' },
	{ id: 'dm-held', name: '박예시', kind: 'dm' as const, myRole: 'member' }
];

const itemNamed = (page: Page, name: string) =>
	page.locator('[data-sidebar="menu-item"]').filter({ hasText: name });

async function holdWithFinger(page: Page, name: string): Promise<void> {
	const box = await itemNamed(page, name).boundingBox();
	if (!box) throw new Error(`the conversation row ${name} has no box to press`);
	const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const session = await page.context().newCDPSession(page);
	await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
	await page.waitForTimeout(900);
	await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

test.describe('on a pointer that can hover', () => {
	test('the more button appears on hover and a right click opens the same menu', async ({ page }) => {
		await mockDeviceMessenger(page, reader, conversations, []);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/messenger?channel=channel-open');

		const held = itemNamed(page, '누를 채널');
		await expect(held.getByRole('button', { name: '대화 메뉴' })).toHaveCount(1);

		await held.click({ button: 'right' });
		await expect(page.getByRole('menuitem', { name: '이 대화 알림 끄기' })).toBeVisible();
		await expect(page).toHaveURL(/channel=channel-open/);
	});
});

test.describe('on a touch screen', () => {
	test.use({ hasTouch: true, isMobile: true, viewport: { width: 700, height: 900 } });

	test('no row shows a more button and a long press opens the menu without opening the conversation', async ({
		page
	}) => {
		await mockDeviceMessenger(page, reader, conversations, []);
		await page.goto('/messenger?channel=channel-open');
		await expect(itemNamed(page, '누를 채널')).toBeVisible();

		await expect(page.getByRole('button', { name: '대화 메뉴' })).toHaveCount(0);

		await holdWithFinger(page, '누를 채널');
		await expect(page.getByRole('menuitem', { name: '이 대화 알림 끄기' })).toBeVisible();
		await expect(page).toHaveURL(/channel=channel-open/);

		await page.keyboard.press('Escape');
		await holdWithFinger(page, '박예시');
		await expect(page.getByRole('menuitem', { name: '이 대화 알림 끄기' })).toBeVisible();
		await expect(page).toHaveURL(/channel=channel-open/);
	});
});
