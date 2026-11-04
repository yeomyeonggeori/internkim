import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';
import { phoneWidth } from '../support/screen-widths';

const channelID = 'lightbox-drag-channel';
const picture = '<svg xmlns="http://www.w3.org/2000/svg" width="540" height="960"><rect width="540" height="960" fill="#466baa"/></svg>';
const pictureSource = `data:image/svg+xml,${encodeURIComponent(picture)}`;

async function openPicture(page: Page) {
	await mockDeviceMessenger(page, { id: 'drag-reader', email: 'reader@example.com' }, [{ id: channelID, name: '사진 대화', kind: 'group', myRole: 'member' }], [
		{
			id: 'drag-picture', sender: { id: 'drag-author', name: '이샘플' }, text: '', sentAt: '2026-10-06T02:00:00Z',
			attachments: [{ kind: 'image', url: pictureSource, source: pictureSource, filename: 'sample-photo.svg', widthPixels: 540, heightPixels: 960 }]
		}
	]);
	await page.route('**/agent/api/person-pictures', route => route.fulfill({ json: {} }));
	await page.route('**/agent/api/custom-emoji', route => route.fulfill({ json: { emojis: [] } }));
	await page.goto(`/messenger?channel=${channelID}`);
	await page.getByRole('button', { name: 'sample-photo.svg', exact: true }).click();
	const frame = page.getByRole('dialog').locator('[data-loading-image]');
	await expect(frame).toHaveAttribute('data-loading-image', 'loaded');
	return frame;
}

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
	const frame = await openPicture(page);
	const box = await frame.boundingBox();
	if (!box) throw new Error('The lightbox picture has no box');
	await dragDown(page, { x: box.x + box.width / 2, y: box.y + box.height / 2 }, 220);
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'sample-photo.svg', exact: true })).toBeVisible();
});

test('a short drag puts the picture back where it was', async ({ page }) => {
	const frame = await openPicture(page);
	const before = await frame.boundingBox();
	if (!before) throw new Error('The lightbox picture has no box');
	await dragDown(page, { x: before.x + before.width / 2, y: before.y + before.height / 2 }, 40);
	await expect(frame).toBeVisible();
	await expect.poll(async () => (await frame.boundingBox())?.y).toBeCloseTo(before.y, 0);
});
