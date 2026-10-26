import { expect, test, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-attachment-download';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const storeOrigin = 'https://store.example.com';
const pictureName = '회의 사진.png';
const fileName = '회의록 초안.pdf';
const onePixelPNG = Buffer.from(
	'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
	'base64'
);

const pictureMessage = {
	id: 'message-picture',
	sender: author,
	text: '',
	sentAt: '2026-10-07T01:00:00Z',
	attachments: [
		{
			kind: 'image',
			url: 'meeting-picture',
			source: `${storeOrigin}/object/sign/asset/meeting-picture.png?token=sample`,
			filename: pictureName,
			widthPixels: 120,
			heightPixels: 120
		}
	]
};

const fileMessage = {
	id: 'message-file',
	sender: author,
	text: '',
	sentAt: '2026-10-07T01:05:00Z',
	attachments: [
		{
			kind: 'file',
			url: 'meeting-notes',
			source: `${storeOrigin}/object/sign/asset/meeting-notes.pdf?token=sample`,
			filename: fileName,
			mimeType: 'application/pdf',
			sizeBytes: 2048
		}
	]
};

async function mockStore(page: Page): Promise<void> {
	await page.route(`${storeOrigin}/**`, async (route) => {
		const downloadAs = new URL(route.request().url()).searchParams.get('download');
		const headers: Record<string, string> = downloadAs
			? { 'content-disposition': `attachment; filename*=UTF-8''${encodeURIComponent(downloadAs)}` }
			: {};
		await route.fulfill({ body: onePixelPNG, contentType: 'image/png', headers });
	});
}

async function settledBox(element: Locator) {
	await element.evaluate((node) => Promise.all(node.getAnimations().map((animation) => animation.finished)));
	return element.boundingBox();
}

test.describe('messenger attachment download', () => {
	test.beforeEach(async ({ page }) => {
		await mockStore(page);
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: channelID, name: '첨부 채널', kind: 'group', myRole: 'member' }],
			[pictureMessage, fileMessage]
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);
	});

	test('the viewer downloads the picture under its own name and zooms it', async ({ page }) => {
		await page.getByRole('button', { name: pictureName }).click();
		const viewer = page.getByRole('dialog');
		await expect(viewer.getByText(pictureName)).toBeVisible();

		const downloading = page.waitForEvent('download');
		await viewer.getByRole('button', { name: '다운로드' }).click();
		expect((await downloading).suggestedFilename()).toBe(pictureName);

		await expect(viewer.getByText('100%')).toBeVisible();
		await viewer.getByRole('button', { name: '확대' }).click();
		await expect(viewer.getByText('110%')).toBeVisible();
	});

	test('the viewer closes on Escape', async ({ page }) => {
		await page.getByRole('button', { name: pictureName }).click();
		await expect(page.getByRole('dialog')).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('dialog')).toHaveCount(0);
	});

	test('the viewer picture menu downloads the picture under its own name', async ({ page }) => {
		await page.getByRole('button', { name: pictureName }).click();
		const viewer = page.getByRole('dialog');
		await viewer.locator('img').click({ button: 'right' });
		const downloadItem = page.getByRole('menuitem', { name: '사진 다운로드' });
		await expect(page.getByRole('menuitem', { name: '사진 복사' })).toBeVisible();
		const downloading = page.waitForEvent('download');
		await downloadItem.click();
		expect((await downloading).suggestedFilename()).toBe(pictureName);
	});

	test('the viewer picture menu keeps its size when the picture is zoomed', async ({ page }) => {
		await page.getByRole('button', { name: pictureName }).click();
		const viewer = page.getByRole('dialog');
		const picture = viewer.locator('img');
		await picture.click({ button: 'right' });
		const unzoomed = await settledBox(page.getByRole('menu'));
		await page.keyboard.press('Escape');
		await expect(page.getByRole('menu')).toHaveCount(0);

		await viewer.getByRole('button', { name: '확대' }).click();
		await expect(viewer.getByText('110%')).toBeVisible();
		await picture.click({ button: 'right' });
		const zoomed = await settledBox(page.getByRole('menu'));
		if (!unzoomed || !zoomed) throw new Error('the picture menu has no layout box');
		expect(zoomed.width).toBeCloseTo(unzoomed.width, 0);
	});

	test('the message menu downloads the picture under its own name', async ({ page }) => {
		await page.locator('img[data-message-picture]').click({ button: 'right' });
		const downloading = page.waitForEvent('download');
		await page.getByRole('menuitem', { name: '사진 다운로드' }).click();
		expect((await downloading).suggestedFilename()).toBe(pictureName);
	});

	test('a file card downloads the file under its own name', async ({ page }) => {
		const downloading = page.waitForEvent('download');
		await page.getByRole('button', { name: '다운로드' }).click();
		expect((await downloading).suggestedFilename()).toBe(fileName);
	});
});
