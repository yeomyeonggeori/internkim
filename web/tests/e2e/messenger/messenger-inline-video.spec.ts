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

	test('a video message shows its controls and seeks where the bar is moved', async ({ page }) => {
		const player = page.locator('[data-video-player="message"]');
		await expect(player.getByRole('button', { name: '재생', exact: true })).toBeVisible();
		await expect(player.getByText('00:04')).toBeVisible();

		const position = player.getByLabel('재생 위치').getByRole('slider');
		await position.focus();
		await page.keyboard.press('End');
		await expect(position).toHaveAttribute('aria-valuenow', /^[34]/);
		await expect.poll(() => playbackSecondsOf(player)).toBeGreaterThan(3);
	});

	test('a video opens in the viewer with the same controls and a download button', async ({ page }) => {
		await page.getByRole('button', { name: '영상 크게 보기' }).click();
		const viewer = page.getByRole('dialog');
		await expect(viewer.getByText(videoName)).toBeVisible();
		await expect(viewer.getByRole('button', { name: '다운로드' })).toBeVisible();
		await expect(viewer.getByRole('button', { name: '확대' })).toHaveCount(0);

		const player = viewer.locator('[data-video-player="viewer"]');
		await expect(player.getByLabel('소리 크기').getByRole('slider')).toBeVisible();
		await player.getByRole('button', { name: '재생 속도' }).click();
		await page.getByRole('menuitemradio', { name: '1.5x' }).click();
		await expect(player.getByRole('button', { name: '재생 속도' })).toHaveText('1.5x');
		await player.getByRole('button', { name: '재생', exact: true }).first().click();
		await expect.poll(() => playbackSecondsOf(player)).toBeGreaterThan(0);
	});
});
