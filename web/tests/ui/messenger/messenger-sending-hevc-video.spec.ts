import { test } from '@playwright/test';
import { expectRelayVideo, sendFixtureVideo } from './messenger-video-send';

test.use({ channel: 'chrome' });

test('an HEVC video is sent re-encoded as H.264 MP4 in Google Chrome', async ({ page }) => {
	expectRelayVideo(await sendFixtureVideo(page, 'sample-hevc.mp4', 'video/mp4'));
});
