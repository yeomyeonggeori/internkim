import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-sending-attachments';
const reader = { id: 'person-reader', email: 'reader@example.com' };
const onePixelPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=';

test('attachments still uploading show as attachments, never as their file names in a bubble', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '첨부 채널', kind: 'group', myRole: 'member' }], []);
	let releaseUpload: () => void = () => {};
	const uploadReleased = new Promise<void>((resolve) => (releaseUpload = resolve));
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') await uploadReleased;
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages: [], hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.goto(`/messenger?channel=${channelID}`);

	await page.locator('input[type="file"]').first().setInputFiles([
		{ name: 'photo.png', mimeType: 'image/png', buffer: Buffer.from(onePixelPNG, 'base64') },
		{ name: 'notes.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4') }
	]);
	await page.getByRole('button', { name: '보내기' }).click();

	const sending = page.locator('[data-message-id^="pending-"]');
	await expect(sending.getByRole('status', { name: '올리는 중' })).toBeVisible();
	await expect(sending.getByText('notes.pdf')).toBeVisible();
	await expect(sending.getByText('올리는 중')).toBeVisible();
	await expect(sending.locator('[data-slot="bubble"]')).toHaveCount(0);
	await expect(sending.getByText('photo.png')).toHaveCount(0);

	releaseUpload();
	await expect(sending.getByRole('status', { name: '올리는 중' })).toHaveCount(0);
});
