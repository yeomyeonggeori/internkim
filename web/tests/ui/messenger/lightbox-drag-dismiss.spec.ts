import { expect, test, type Page } from '@playwright/test';
import { phoneWidth } from '../support/screen-widths';
import { openLightboxPicture } from './lightbox-picture';

async function dragDown(page: Page, from: { x: number; y: number }, distance: number) {
	const session = await page.context().newCDPSession(page);
	await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [from] });
	const steps = 10;
	for (let index = 1; index <= steps; index += 1) {
		await session.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: from.x, y: from.y + (distance * index) / steps }] });
		await page.waitForTimeout(30);
	}
	await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

test.use({ locale: 'ko-KR', hasTouch: true, isMobile: true, viewport: { width: phoneWidth, height: 844 } });

test('dragging a picture far enough down closes the viewer and returns to the conversation', async ({ page }) => {
	const frame = await openLightboxPicture(page);
	const box = await frame.boundingBox();
	if (!box) throw new Error('The lightbox picture has no box');
	await dragDown(page, { x: box.x + box.width / 2, y: box.y + box.height / 2 }, 220);
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'sample-photo.svg', exact: true })).toBeVisible();
});

test('a short drag puts the picture back where it was', async ({ page }) => {
	const frame = await openLightboxPicture(page);
	const before = await frame.boundingBox();
	if (!before) throw new Error('The lightbox picture has no box');
	await dragDown(page, { x: before.x + before.width / 2, y: before.y + before.height / 2 }, 40);
	await expect(frame).toBeVisible();
	await expect.poll(async () => (await frame.boundingBox())?.y).toBeCloseTo(before.y, 0);
});
