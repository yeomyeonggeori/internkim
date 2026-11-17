import { expect, test, type Page } from '@playwright/test';
import { desktopWidth, phoneWidth } from '../support/screen-widths';
import { openLightboxPicture } from './lightbox-picture';

async function spreadTwoFingers(page: Page, center: { x: number; y: number }, fromGap: number, toGap: number) {
	const session = await page.context().newCDPSession(page);
	const fingersAt = (gap: number) => [{ x: center.x, y: center.y - gap / 2, id: 1 }, { x: center.x, y: center.y + gap / 2, id: 2 }];
	await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: fingersAt(fromGap) });
	const steps = 10;
	for (let index = 1; index <= steps; index += 1) {
		await session.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: fingersAt(fromGap + ((toGap - fromGap) * index) / steps) });
		await page.waitForTimeout(30);
	}
	await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
}

test.describe('on a desktop', () => {
	test.use({ locale: 'ko-KR', viewport: { width: desktopWidth, height: 900 } });

	test('pinching on the trackpad zooms the picture in and back out', async ({ page }) => {
		const frame = await openLightboxPicture(page);
		const box = await frame.boundingBox();
		if (!box) throw new Error('The lightbox picture has no box');
		const zoomLabel = page.getByRole('dialog').getByText(/^\d+%$/);
		await expect(zoomLabel).toHaveText('100%');

		await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
		await page.keyboard.down('Control');
		await page.mouse.wheel(0, -60);
		await expect.poll(async () => Number.parseInt(await zoomLabel.innerText(), 10)).toBeGreaterThan(100);
		await page.mouse.wheel(0, 400);
		await page.keyboard.up('Control');
		await expect(zoomLabel).toHaveText('100%');
	});
});

test.describe('on a phone', () => {
	test.use({ locale: 'ko-KR', hasTouch: true, isMobile: true, viewport: { width: phoneWidth, height: 844 } });

	test('spreading two fingers enlarges the picture and keeps the viewer on it', async ({ page }) => {
		const frame = await openLightboxPicture(page);
		const before = await frame.boundingBox();
		if (!before) throw new Error('The lightbox picture has no box');

		await spreadTwoFingers(page, { x: before.x + before.width / 2, y: before.y + before.height / 2 }, 80, 200);

		await expect.poll(async () => (await frame.boundingBox())?.width ?? 0).toBeGreaterThan(before.width * 1.5);
		await expect(page.getByRole('dialog')).toBeVisible();
	});
});
