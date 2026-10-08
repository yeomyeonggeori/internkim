import { expect, test, type Locator, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };

const longText =
	'다음 주 일정은 화요일 오후 2시로 잡았습니다. 장소는 3층 회의실이고, 참석이 어려우신 분은 미리 말씀해 주시면 온라인 링크를 따로 보내드리겠습니다.';

function avatarOf(page: Page, messageID: string): Locator {
	return page.locator(`[data-message-id="${messageID}"] [data-slot="sender-avatar"]`);
}

function bubbleOf(page: Page, messageID: string): Locator {
	return page.locator(`[data-message-id="${messageID}"] [data-slot="bubble-content"]`).first();
}

async function boxOf(locator: Locator): Promise<{ y: number; height: number }> {
	const box = await locator.boundingBox();
	if (!box) throw new Error('the element has no box to measure');
	return box;
}

async function openConversation(page: Page, messages: object[]): Promise<void> {
	await mockDeviceMessenger(page, reader, [{ id: 'channel-avatar', name: '사진 채널', kind: 'group', myRole: 'member' }], messages);
	await page.setViewportSize({ width: 390, height: 760 });
	await page.goto('/messenger?channel=channel-avatar');
	await expect(page.locator(`[data-message-id="last"]`)).toBeVisible();
}

test('the sender picture sits beside the first message of a group, centered on a one-line bubble', async ({ page }) => {
	await openConversation(page, [
		{ id: 'first', sender: author, text: '안녕하세요.', sentAt: '2026-10-07T01:01:00Z' },
		{ id: 'last', sender: author, text: '확인 부탁드려요.', sentAt: '2026-10-07T01:01:10Z' }
	]);

	await expect(avatarOf(page, 'first')).toBeVisible();
	await expect(avatarOf(page, 'last')).toHaveCount(0);

	const avatar = await boxOf(avatarOf(page, 'first'));
	const bubble = await boxOf(bubbleOf(page, 'first'));
	expect(Math.abs(avatar.y + avatar.height / 2 - (bubble.y + bubble.height / 2))).toBeLessThanOrEqual(1);
});

test('beside a long first message the picture stays on the top line of the bubble', async ({ page }) => {
	await openConversation(page, [
		{ id: 'first', sender: author, text: longText, sentAt: '2026-10-07T01:01:00Z' },
		{ id: 'last', sender: author, text: '확인 부탁드려요.', sentAt: '2026-10-07T01:01:10Z' }
	]);

	const avatar = await boxOf(avatarOf(page, 'first'));
	const bubble = await boxOf(bubbleOf(page, 'first'));
	expect(bubble.height).toBeGreaterThan(avatar.height * 2);
	expect(avatar.y).toBeGreaterThanOrEqual(bubble.y);
	expect(avatar.y - bubble.y).toBeLessThanOrEqual(8);
});

test('a message with a file shows its text above the file', async ({ page }) => {
	await openConversation(page, [
		{
			id: 'last',
			sender: author,
			text: '회의록 공유드립니다.',
			sentAt: '2026-10-07T01:01:00Z',
			attachments: [
				{
					kind: 'file',
					url: 'meeting-notes',
					source: 'https://store.example.com/object/sign/asset/meeting-notes.pdf?token=sample',
					filename: '회의록 초안.pdf',
					mimeType: 'application/pdf',
					sizeBytes: 2048
				}
			]
		}
	]);

	const row = page.locator('[data-message-id="last"]');
	const bubble = await boxOf(bubbleOf(page, 'last'));
	const file = await boxOf(row.getByText('회의록 초안.pdf'));
	expect(file.y).toBeGreaterThan(bubble.y + bubble.height);
	await expect(row.locator('time')).toHaveCount(1);
});
