import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-sending-in-a-row';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };

async function mockChannelWhoseReadsStallAfterSending(page: Page): Promise<string[]> {
	const messages: object[] = [];
	const delivered: string[] = [];
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: channelID, name: '연속 전송 채널', kind: 'group', myRole: 'member' }],
		messages
	);
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') {
			const { message } = route.request().postDataJSON() as { message: string };
			delivered.push(message);
			await route.fulfill({
				json: { conversationID: channelID, currentUserId: reader.id, messages, hasMoreBefore: false, historyCursor: '' }
			});
			return;
		}
		if (delivered.length > 0) return;
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages, hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);
	return delivered;
}

async function send(page: Page, written: string): Promise<void> {
	await page.getByRole('combobox', { name: '메시지를 입력하세요' }).fill(written);
	await page.getByRole('button', { name: '보내기' }).click();
}

test('the next message sends before the conversation is read again', async ({ page }) => {
	const delivered = await mockChannelWhoseReadsStallAfterSending(page);

	await send(page, '첫 메시지');
	await expect.poll(() => delivered).toEqual(['첫 메시지']);

	await send(page, '둘째 메시지');
	await expect.poll(() => delivered).toEqual(['첫 메시지', '둘째 메시지']);
});
