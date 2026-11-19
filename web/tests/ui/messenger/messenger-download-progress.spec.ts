import { expect, test, type Page } from '@playwright/test';
import {
	insideTheAndroidApp,
	insideTheIPhoneApp,
	insideTheMacApp,
	invocationsOf,
	refuseCommand,
	refuseCommandWith,
	reportNativeDownloadProgress,
	withNativeDownloading,
	withTheDownloadPlugins
} from '../support/iphone-app';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-download-progress';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const storeOrigin = 'https://store.example.com';
const pictureName = '회의 사진.png';
const fileName = '회의록 초안.pdf';
const onePixelPNG = Buffer.from(
	'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
	'base64'
);
const halfwayProgress = { receivedBytes: 10_171_187, totalBytes: 24_222_105 };

const messages = [
	{
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
	},
	{
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
				sizeBytes: 24_222_105
			}
		]
	}
];

type HeldStore = { release: () => void };

async function holdTheStore(page: Page): Promise<HeldStore> {
	let release = () => {};
	const released = new Promise<void>((resolve) => (release = resolve));
	await page.route(`${storeOrigin}/**`, async (route) => {
		const isPicture = new URL(route.request().url()).pathname.endsWith('.png');
		if (route.request().resourceType() !== 'image') await released;
		await route.fulfill(isPicture ? { body: onePixelPNG, contentType: 'image/png' } : { body: 'notes', contentType: 'application/pdf' });
	});
	return { release };
}

async function openTheConversation(page: Page): Promise<void> {
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '첨부 채널', kind: 'group', myRole: 'member' }], messages);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);
}

function fileCard(page: Page) {
	return page.locator('[data-slot="attachment"]').filter({ hasText: fileName });
}

test.describe('a download inside the iPhone app that downloads by itself', () => {
	test.beforeEach(async ({ page }) => {
		await insideTheIPhoneApp(page);
		await withTheDownloadPlugins(page);
		await withNativeDownloading(page);
	});

	test('the file card becomes a ring, counts what arrived, and starts one download however often it is tapped', async ({ page }) => {
		const store = await holdTheStore(page);
		await openTheConversation(page);
		await reportNativeDownloadProgress(page, halfwayProgress);

		const download = fileCard(page).getByRole('button', { name: '다운로드' });
		await download.click();
		await expect(fileCard(page).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42');
		await expect(fileCard(page).getByText('받는 중 · 9.7 / 23.1 MB')).toBeVisible();
		await download.click();
		await download.click();
		expect(await invocationsOf(page, 'plugin:file-download|download')).toHaveLength(1);

		store.release();
		await expect.poll(async () => invocationsOf(page, 'plugin:share-sheet|present_download')).toHaveLength(1);
		const [handedOver] = await invocationsOf(page, 'plugin:share-sheet|present_download');
		expect(handedOver).toMatchObject({ mediaType: 'application/pdf' });
		await expect(fileCard(page).getByRole('img', { name: '받았습니다' })).toBeVisible();
		await expect(fileCard(page).getByRole('progressbar')).toHaveCount(0);
		await expect(fileCard(page).getByText('PDF · 23.1 MB')).toBeVisible();
		await expect(page.locator('[data-sonner-toast]')).toHaveCount(0);
		expect(await invocationsOf(page, 'plugin:share-sheet|present')).toEqual([]);
	});

	test('the viewer shows the ring, and the picture keeps it once the viewer is closed', async ({ page }) => {
		const store = await holdTheStore(page);
		await openTheConversation(page);
		await reportNativeDownloadProgress(page, halfwayProgress);

		await page.getByRole('button', { name: pictureName }).click();
		const viewer = page.getByRole('dialog');
		await viewer.getByRole('button', { name: '다운로드' }).click();
		await expect(viewer.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42');
		await page.keyboard.press('Escape');
		await expect(viewer).toHaveCount(0);

		const badge = page.locator('[data-download-badge]');
		await expect(badge.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42');
		store.release();
		await expect(page.getByText('사진함에 저장했습니다.')).toBeVisible();
		const [saved] = await invocationsOf(page, 'plugin:media-save|save_download');
		expect(saved).toMatchObject({ mediaType: 'image/png' });
		await expect(badge).toHaveCount(0);
		expect(await invocationsOf(page, 'plugin:file-download|download')).toHaveLength(1);
	});

	test('the message menu leaves a ring on the picture it downloads', async ({ page }) => {
		const store = await holdTheStore(page);
		await openTheConversation(page);

		await page.locator('img[data-message-picture]').click({ button: 'right' });
		await page.getByRole('menuitem', { name: '사진 다운로드' }).click();
		await expect(page.locator('[data-download-badge]').getByRole('progressbar')).toBeVisible();
		store.release();
		await expect(page.getByText('사진함에 저장했습니다.')).toBeVisible();
	});

	test('a refused photo permission says to allow it in Settings and gives the button back', async ({ page }) => {
		await holdTheStore(page).then((store) => store.release());
		await openTheConversation(page);
		await refuseCommandWith(page, 'plugin:media-save|save_download', { reason: 'photoPermission', message: 'not allowed' });

		await page.getByRole('button', { name: pictureName }).click();
		const viewer = page.getByRole('dialog');
		await viewer.getByRole('button', { name: '다운로드' }).click();

		await expect(page.getByText('저장하지 못했습니다. 설정에서 사진 추가를 허용해 주세요.')).toBeVisible();
		await expect(viewer.getByRole('progressbar')).toHaveCount(0);
		await expect(viewer.getByRole('img', { name: '받았습니다' })).toHaveCount(0);
	});

	test('a download that fails asks to try again', async ({ page }) => {
		await holdTheStore(page).then((store) => store.release());
		await openTheConversation(page);
		await refuseCommand(page, 'plugin:file-download|download');

		await fileCard(page).getByRole('button', { name: '다운로드' }).click();

		await expect(page.getByText('저장하지 못했습니다. 다시 시도해 주세요.')).toBeVisible();
		await expect(fileCard(page).getByRole('progressbar')).toHaveCount(0);
	});
});

test.describe('a download inside the Android app that downloads by itself', () => {
	test('a picture lands in the gallery and says so', async ({ page }) => {
		await insideTheAndroidApp(page);
		await withTheDownloadPlugins(page);
		await withNativeDownloading(page);
		await holdTheStore(page).then((store) => store.release());
		await openTheConversation(page);

		await page.getByRole('button', { name: pictureName }).click();
		await page.getByRole('dialog').getByRole('button', { name: '다운로드' }).click();

		await expect(page.getByText('갤러리에 저장했습니다.')).toBeVisible();
	});
});

test.describe('a download inside an iPhone app built before it could download by itself', () => {
	test('the page fetches the file and hands it over in chunks, with the ring', async ({ page }) => {
		await insideTheIPhoneApp(page);
		await withTheDownloadPlugins(page);
		const store = await holdTheStore(page);
		await openTheConversation(page);

		await fileCard(page).getByRole('button', { name: '다운로드' }).click();
		await expect(fileCard(page).getByRole('progressbar')).toBeVisible();
		store.release();

		await expect.poll(async () => (await invocationsOf(page, 'plugin:share-sheet|present')).length).toBe(1);
		expect(await invocationsOf(page, 'plugin:file-download|download')).toEqual([]);
		expect(await invocationsOf(page, 'plugin:share-sheet|present_download')).toEqual([]);
	});
});

test.describe('a download inside the Mac app', () => {
	test('the file card shows the ring and the download says it was saved', async ({ page }) => {
		await insideTheMacApp(page);
		const store = await holdTheStore(page);
		await openTheConversation(page);

		await fileCard(page).getByRole('button', { name: '다운로드' }).click();
		await expect(fileCard(page).getByRole('progressbar')).toBeVisible();
		store.release();

		await expect(page.getByText('저장했습니다.', { exact: true })).toBeVisible();
		expect(await invocationsOf(page, 'plugin:file-download|download')).toEqual([]);
	});
});

test.describe('a download in the browser', () => {
	test('the browser downloads the file itself and the page shows no ring it cannot fill', async ({ page }) => {
		await holdTheStore(page).then((store) => store.release());
		await openTheConversation(page);

		const downloading = page.waitForEvent('download');
		await fileCard(page).getByRole('button', { name: '다운로드' }).click();
		await downloading;
		await expect(fileCard(page).getByRole('progressbar')).toHaveCount(0);
	});
});
