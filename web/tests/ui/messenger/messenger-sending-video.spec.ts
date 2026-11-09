import { expect, test } from '@playwright/test';
import { attachFile, expectRelayVideo, openVideoChannel, sendFixtureVideo } from './messenger-video-send';

test('a QuickTime video is sent as an H.264 MP4 the relay stores', async ({ page }) => {
	expectRelayVideo(await sendFixtureVideo(page, 'sample-h264.mov', 'video/quicktime'));
});

test('a video the browser cannot convert says so and holds the send button', async ({ page }) => {
	await openVideoChannel(page);
	await attachFile(page, 'broken.mp4', 'video/mp4', Buffer.from('not a video'));

	await expect(page.getByText('보낼 수 없는 파일이에요')).toBeVisible();
	await expect(page.getByRole('button', { name: '보내기' })).toBeDisabled();

	await page.getByRole('button', { name: '첨부 제거' }).click();
	await expect(page.getByText('broken.mp4')).toHaveCount(0);
});
