import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-failed-send';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const idleRefreshMs = 5000;

type Delivery = { failNext: number };

async function mockChannelThatFailsToSend(page: Page, delivery: Delivery): Promise<void> {
	const messages: object[] = [
		{ id: 'message-root', sender: author, text: '스레드 원글', sentAt: '2026-09-28T01:00:00Z' }
	];
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: channelID, name: '전송 채널', kind: 'group', myRole: 'member' }],
		messages
	);
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') {
			if (delivery.failNext > 0) {
				delivery.failNext--;
				await route.fulfill({ status: 503, body: 'relay unavailable' });
				return;
			}
			const { message, replyToRootId } = route.request().postDataJSON() as {
				message: string;
				replyToRootId?: string;
			};
			messages.push({
				id: `message-${messages.length}`,
				threadRootId: replyToRootId,
				sender: reader,
				text: message,
				sentAt: '2026-09-28T01:05:00Z'
			});
		}
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages, hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);
}

async function send(page: Page, composerName: string, written: string): Promise<void> {
	await page.getByRole('combobox', { name: composerName }).fill(written);
	await page.getByRole('button', { name: '보내기' }).click();
}

test('a message that failed to send survives a refresh and sends again', async ({ page }) => {
	await page.clock.install();
	const delivery = { failNext: 1 };
	await mockChannelThatFailsToSend(page, delivery);

	await send(page, '메시지를 입력하세요', '실패할 메시지');

	const failure = page.getByRole('group', { name: '보내지 못했어요' });
	await expect(failure).toBeVisible();
	await expect(page.getByText('실패할 메시지')).toBeVisible();

	await page.clock.fastForward(idleRefreshMs * 2);
	await expect(failure).toBeVisible();

	await failure.getByRole('button', { name: '다시 전송' }).click();
	await expect(failure).toBeHidden();
	await expect(page.locator('[data-message-id^="message-"]').filter({ hasText: '실패할 메시지' })).toBeVisible();
});

test('deleting a message that failed to send removes it without calling the server', async ({ page }) => {
	const delivery = { failNext: 1 };
	await mockChannelThatFailsToSend(page, delivery);
	let postsAfterFailure = 0;

	await send(page, '메시지를 입력하세요', '지울 메시지');
	const failure = page.getByRole('group', { name: '보내지 못했어요' });
	await expect(failure).toBeVisible();

	page.on('request', (request) => {
		if (request.method() === 'POST' && request.url().includes('/agent/api/dm')) postsAfterFailure++;
	});
	await failure.getByRole('button', { name: '삭제' }).click();

	await expect(page.getByText('지울 메시지')).toBeHidden();
	expect(postsAfterFailure).toBe(0);
});

test('a thread reply that failed to send offers the same choices', async ({ page }) => {
	const delivery = { failNext: 1 };
	await mockChannelThatFailsToSend(page, delivery);

	await page.getByText('스레드 원글').click();
	const threadSheet = page.getByRole('dialog', { name: '글타래' });
	await expect(threadSheet).toBeVisible();

	await threadSheet.getByRole('combobox', { name: '답글을 입력하세요' }).fill('실패할 답글');
	await threadSheet.getByRole('button', { name: '보내기' }).click();

	const failure = threadSheet.getByRole('group', { name: '보내지 못했어요' });
	await expect(failure).toBeVisible();
	await failure.getByRole('button', { name: '다시 전송' }).click();
	await expect(failure).toBeHidden();
	await expect(threadSheet.locator('[data-message-id^="message-"]').filter({ hasText: '실패할 답글' })).toBeVisible();
});
