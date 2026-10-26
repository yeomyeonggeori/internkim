import { readFileSync } from 'node:fs';
import { expect, test, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-inline-video';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const storeOrigin = 'https://store.example.com';
const videoName = '회의 영상.webm';
const sampleVideo = readFileSync(new URL('./fixtures/sample-video.webm', import.meta.url));

const videoMessage = {
	id: 'message-video',
	sender: author,
	text: '',
	sentAt: '2026-10-07T01:00:00Z',
	attachments: [
		{
			kind: 'video',
			url: 'meeting-video',
			source: `${storeOrigin}/object/sign/asset/meeting-video.webm?token=sample`,
			filename: videoName,
			mimeType: 'video/webm',
			widthPixels: 320,
			heightPixels: 180
		}
	]
};

async function mockStore(page: Page): Promise<void> {
	await page.route(`${storeOrigin}/**`, async (route) => {
		const range = /bytes=(\d+)-(\d*)/.exec(route.request().headers()['range'] ?? '');
		if (!range) {
			await route.fulfill({ body: sampleVideo, contentType: 'video/webm', headers: { 'accept-ranges': 'bytes' } });
			return;
		}
		const first = Number(range[1]);
		const last = range[2] ? Number(range[2]) : sampleVideo.length - 1;
		await route.fulfill({
			status: 206,
			body: sampleVideo.subarray(first, last + 1),
			contentType: 'video/webm',
			headers: { 'accept-ranges': 'bytes', 'content-range': `bytes ${first}-${last}/${sampleVideo.length}` }
		});
	});
}

async function playbackSecondsOf(player: Locator): Promise<number> {
	return player.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
}

test.describe('messenger inline video', () => {
	test.beforeEach(async ({ page }) => {
		await mockStore(page);
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: channelID, name: '영상 채널', kind: 'group', myRole: 'member' }],
			[videoMessage]
		);
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);
	});

	test('a video message plays in place with the browser controls', async ({ page }) => {
		const player = page.locator('[data-video-player="message"]');
		const video = player.locator('video');
		await expect(video).toHaveAttribute('controls');
		await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.duration)).toBeGreaterThan(3);

		await video.click();
		await expect.poll(() => playbackSecondsOf(player)).toBeGreaterThan(0);
		await expect(page.getByRole('dialog')).toHaveCount(0);
	});

	test('a video opens in the viewer with the browser controls and a download button', async ({ page }) => {
		await page.getByRole('button', { name: '영상 크게 보기' }).click();
		const viewer = page.getByRole('dialog');
		await expect(viewer.getByText(videoName)).toBeVisible();
		await expect(viewer.getByRole('button', { name: '다운로드' })).toBeVisible();
		await expect(viewer.getByRole('button', { name: '확대' })).toHaveCount(0);

		const player = viewer.locator('[data-video-player="viewer"]');
		await expect(player.locator('video')).toHaveAttribute('controls');
		await expect(player.getByRole('button', { name: '영상 크게 보기' })).toHaveCount(0);
		await player.locator('video').click();
		await expect.poll(() => playbackSecondsOf(player)).toBeGreaterThan(0);
	});
});

test('arrow keys on a focused video in the viewer stay with the video', async ({ page }) => {
	const pictureName = '회의 사진.gif';
	const pictureAndVideo = {
		...videoMessage,
		attachments: [
			{
				kind: 'image',
				url: 'meeting-picture',
				source: 'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==',
				filename: pictureName,
				mimeType: 'image/gif',
				widthPixels: 1,
				heightPixels: 1
			},
			...videoMessage.attachments
		]
	};
	await mockStore(page);
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: channelID, name: '영상 채널', kind: 'group', myRole: 'member' }],
		[pictureAndVideo]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);

	await page.getByRole('button', { name: pictureName }).click();
	const viewer = page.getByRole('dialog');
	await page.keyboard.press('ArrowRight');
	await expect(viewer.getByText(videoName)).toBeVisible();

	await viewer.locator('[data-video-player="viewer"] video').focus();
	await page.keyboard.press('ArrowRight');
	await expect(viewer.getByText(videoName)).toBeVisible();
	await expect(viewer.locator('[data-video-player="viewer"] video')).toBeVisible();
});
