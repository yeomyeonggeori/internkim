import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'attachment-tiles-channel';
const reader = { id: 'tiles-reader', email: 'reader@example.com' };
const author = { id: 'tiles-author', name: '이샘플' };
const picture = '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200"><rect width="200" height="200" fill="#466baa"/></svg>';
const pictureURL = '/attachment-tiles-fixture/arrived.svg';

test.use({ locale: 'ko-KR', colorScheme: 'dark' });

test('pictures share one square size whether they arrived or not, and files list below them', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 800 });
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '첨부 대화', kind: 'group', myRole: 'member' }], [
		{
			id: 'tiles-message',
			sender: author,
			text: '',
			sentAt: '2026-10-07T01:00:00Z',
			attachments: [
				{ kind: 'image', url: 'not-yet-arrived', filename: '7b67c0fd7f360cc5.png' },
				{ kind: 'image', url: pictureURL, source: pictureURL, filename: 'arrived.svg' },
				{ kind: 'file', url: 'manual', source: '/attachment-tiles-fixture/manual.pdf', filename: 'manual.pdf', mimeType: 'application/pdf', sizeBytes: 2048 }
			]
		}
	]);
	await page.route(`**${pictureURL}`, (route) => route.fulfill({ contentType: 'image/svg+xml', body: picture }));
	await page.goto(`/messenger?channel=${channelID}`);

	const cards = page.locator('[data-message-id="tiles-message"] [data-slot="attachment"]');
	await expect(cards).toHaveCount(3);
	await expect(cards.nth(1).getByRole('img', { name: 'arrived.svg' })).toBeVisible();
	await expect(cards.nth(0).getByText('7b67c0fd7f360cc5.png')).toHaveCount(0);

	const [waiting, arrived, file] = await Promise.all([0, 1, 2].map(async (index) => (await cards.nth(index).boundingBox())!));
	expect(waiting.width).toBeCloseTo(arrived.width, 0);
	expect(waiting.height).toBeCloseTo(arrived.height, 0);
	expect(arrived.height).toBeCloseTo(arrived.width, 0);
	expect(file.y).toBeGreaterThanOrEqual(arrived.y + arrived.height);
	expect(file.x).toBeCloseTo(waiting.x, 0);
	if (process.env.TILES_SCREENSHOT) await page.screenshot({ path: process.env.TILES_SCREENSHOT });
});
