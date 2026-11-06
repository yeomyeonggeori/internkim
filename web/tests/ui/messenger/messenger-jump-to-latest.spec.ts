import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-jump-to-latest';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const idleRefreshMs = 5000;
const earlierMessageCount = 40;

function sentAtMinute(minute: number): string {
	return new Date(Date.UTC(2026, 9, 2, 1, minute)).toISOString();
}

test('the jump-to-latest button counts messages that arrive while the reader is scrolled up', async ({ page }) => {
	await page.clock.install();
	const messages: object[] = Array.from({ length: earlierMessageCount }, (_, index) => ({
		id: `message-earlier-${index}`,
		sender: author,
		text: `이전 메시지 ${index}`,
		sentAt: sentAtMinute(index)
	}));
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '광장', kind: 'group', myRole: 'member' }], messages);
	await page.route('**/agent/api/dm**', async (route) => {
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages, hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);

	const latestEarlier = page.locator(`[data-message-id="message-earlier-${earlierMessageCount - 1}"]`);
	await expect(latestEarlier).toBeVisible();
	const jumpToLatest = page.getByRole('button', { name: '최신 메시지로' });
	await expect(jumpToLatest).toBeHidden();

	await latestEarlier.hover();
	await page.mouse.wheel(0, -2000);
	await expect(jumpToLatest).toBeVisible();

	messages.push(
		{ id: 'message-mine', sender: reader, text: '내가 보낸 메시지', sentAt: sentAtMinute(50) },
		{ id: 'message-new-1', sender: author, text: '새 메시지 하나', sentAt: sentAtMinute(51) },
		{ id: 'message-new-2', sender: author, text: '새 메시지 둘', sentAt: sentAtMinute(52) }
	);
	await page.clock.fastForward(idleRefreshMs);
	const unseen = page.getByRole('button', { name: '새 메시지 2개' });
	await expect(unseen).toBeVisible();

	const box = await unseen.boundingBox();
	if (box === null) throw new Error('the jump-to-latest button has no layout box to click');
	await page.mouse.move(box.x - 120, box.y + box.height / 2);
	await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 10 });
	await page.mouse.down();
	await page.mouse.up();
	await expect(page.locator('[data-message-id="message-new-2"]')).toBeInViewport();
	await expect(page.getByRole('button', { name: '스레드 닫기' })).toBeHidden();
	await expect(unseen).toBeHidden();
	await expect(jumpToLatest).toBeHidden();
});

test('a thread opens at its latest reply, holds its place for new replies, and follows the reader’s own reply', async ({
	page
}) => {
	await page.clock.install();
	const root = { id: 'message-thread-root', sender: author, text: '스레드 원글', sentAt: sentAtMinute(0) };
	const messages: object[] = [
		root,
		...Array.from({ length: earlierMessageCount }, (_, index) => ({
			id: `reply-earlier-${index}`,
			threadRootId: root.id,
			sender: author,
			text: `이전 답글 ${index}`,
			sentAt: sentAtMinute(index + 1)
		}))
	];
	await mockDeviceMessenger(page, reader, [{ id: channelID, name: '광장', kind: 'group', myRole: 'member' }], messages);
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') {
			const { message, replyToRootId } = route.request().postDataJSON() as { message: string; replyToRootId?: string };
			messages.push({ id: 'reply-mine', threadRootId: replyToRootId, sender: reader, text: message, sentAt: sentAtMinute(55) });
		}
		await route.fulfill({
			json: { conversationID: channelID, currentUserId: reader.id, messages, hasMoreBefore: false, historyCursor: '' }
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto(`/messenger?channel=${channelID}`);

	await page.getByText('스레드 원글').click();
	const threadSheet = page.getByRole('dialog', { name: '글타래' });
	const latestEarlier = threadSheet.locator(`[data-message-id="reply-earlier-${earlierMessageCount - 1}"]`);
	await expect(latestEarlier).toBeInViewport();
	const jumpToLatest = threadSheet.getByRole('button', { name: '최신 메시지로' });
	await expect(jumpToLatest).toBeHidden();

	await latestEarlier.hover();
	await page.mouse.wheel(0, -2000);
	await expect(jumpToLatest).toBeVisible();

	messages.push(
		{ id: 'reply-new-1', threadRootId: root.id, sender: author, text: '새 답글 하나', sentAt: sentAtMinute(51) },
		{ id: 'reply-new-2', threadRootId: root.id, sender: author, text: '새 답글 둘', sentAt: sentAtMinute(52) }
	);
	await page.clock.fastForward(idleRefreshMs);
	await expect(threadSheet.getByRole('button', { name: '새 메시지 2개' })).toBeVisible();
	await expect(threadSheet.locator('[data-message-id="reply-new-2"]')).not.toBeInViewport();

	await threadSheet.getByRole('combobox', { name: '답글을 입력하세요' }).fill('내 답글');
	await threadSheet.getByRole('button', { name: '보내기' }).click();
	await expect(threadSheet.locator('[data-message-id^="reply-"]').filter({ hasText: '내 답글' })).toBeInViewport();
	await expect(threadSheet.getByRole('button', { name: /새 메시지|최신 메시지로/ })).toBeHidden();
});
