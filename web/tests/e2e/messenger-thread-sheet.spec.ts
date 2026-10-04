import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-thread-sheet';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };
const replyText = '스레드 답글을 드래그해서 복사합니다';

const rootMessage = {
	id: 'message-root',
	sender: author,
	text: '스레드 원글',
	sentAt: '2026-09-28T01:00:00Z',
	thread: { replyCount: 1, lastReplyAt: '2026-09-28T01:05:00Z', participants: [reader] }
};

const replyMessage = {
	id: 'message-reply',
	threadRootId: rootMessage.id,
	sender: reader,
	text: replyText,
	sentAt: '2026-09-28T01:05:00Z'
};

const unansweredText = '아직 답글이 없는 메시지입니다';

const unansweredMessage = {
	id: 'message-unanswered',
	sender: author,
	text: unansweredText,
	sentAt: '2026-09-28T01:10:00Z'
};

const pictureMessage = {
	id: 'message-picture',
	sender: author,
	text: '',
	sentAt: '2026-09-28T01:15:00Z',
	attachments: [
		{
			kind: 'image',
			url: 'sample-picture.png',
			source:
				'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
			filename: 'sample-picture.png',
			widthPixels: 120,
			heightPixels: 120
		}
	]
};

function mockThreadChannel(page: Page): Promise<void> {
	return mockDeviceMessenger(
		page,
		reader,
		[{ id: channelID, name: '스레드 채널', kind: 'group', myRole: 'member' }],
		[rootMessage, replyMessage, unansweredMessage, pictureMessage]
	);
}

test.describe('messenger thread sheet', () => {
	test.beforeEach(async ({ page }) => {
		await mockThreadChannel(page);
	});

	test('stays open while text in a thread message is dragged to copy it', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);

		await page.getByRole('button', { name: /1개 답글/ }).click();
		const threadSheet = page.getByRole('dialog', { name: '글타래' });
		const reply = threadSheet.getByText(replyText);
		await expect(reply).toBeVisible();
		await threadSheet.evaluate((sheet) =>
			Promise.all(sheet.getAnimations({ subtree: true }).map((animation) => animation.finished))
		);

		const box = await reply.boundingBox();
		if (box === null) throw new Error('the thread reply has no layout box to drag across');
		await page.mouse.move(box.x + 2, box.y + box.height / 2);
		await page.mouse.down();
		await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2, { steps: 8 });
		await page.mouse.up();

		await expect(threadSheet).toBeVisible();
		await expect(reply).toBeVisible();
		expect(await page.evaluate(() => window.getSelection()?.toString() ?? '')).toContain('답글을 드래그해서');
	});

	test('opens an empty thread when a message with no replies is clicked', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);

		await page.getByText(unansweredText).click();

		const threadSheet = page.getByRole('dialog', { name: '글타래' });
		await expect(threadSheet).toBeVisible();
		await expect(threadSheet.getByText(unansweredText)).toBeVisible();
	});

	test('going back closes the thread and keeps the conversation open', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);
		await page.getByRole('button', { name: /1개 답글/ }).click();
		const threadSheet = page.getByRole('dialog', { name: '글타래' });
		await expect(threadSheet).toBeVisible();

		await page.goBack();

		await expect(threadSheet).toHaveCount(0);
		await expect(page).toHaveURL(new RegExp(`/messenger\\?channel=${channelID}$`));
		await expect(page.getByText(unansweredText)).toBeVisible();
	});

	test('closing the thread leaves no extra step for going back', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);
		await page.getByRole('button', { name: /1개 답글/ }).click();
		const threadSheet = page.getByRole('dialog', { name: '글타래' });
		await expect(threadSheet).toBeVisible();

		await page.keyboard.press('Escape');
		await expect(threadSheet).toHaveCount(0);
		await expect(page).toHaveURL(new RegExp(`/messenger\\?channel=${channelID}$`));

		await page.goBack();

		await expect(page).toHaveURL('about:blank');
	});

	test('opens the picture instead of the thread when a picture in a message is clicked', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);

		await page.getByRole('button', { name: 'sample-picture.png' }).click();

		await expect(page.getByRole('dialog')).toHaveCount(1);
		await expect(page.getByRole('dialog', { name: '글타래' })).toHaveCount(0);
	});

	test('does not open a thread when text in a timeline message is dragged to select it', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto(`/messenger?channel=${channelID}`);

		const message = page.getByText(unansweredText);
		const box = await message.boundingBox();
		if (box === null) throw new Error('the timeline message has no layout box to drag across');
		await page.mouse.move(box.x + 2, box.y + box.height / 2);
		await page.mouse.down();
		await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2, { steps: 8 });
		await page.mouse.up();

		await expect(page.getByRole('dialog', { name: '글타래' })).toHaveCount(0);
		expect(await page.evaluate(() => window.getSelection()?.toString() ?? '')).toContain('답글이 없는');
	});
});

test.describe('messenger thread sheet on a touch screen', () => {
	test.use({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 } });

	test.beforeEach(async ({ page }) => {
		await mockThreadChannel(page);
	});

	test('keeps a long press for the message menu instead of text selection', async ({ page }) => {
		await page.goto(`/messenger?channel=${channelID}`);

		const reply = page.getByText(replyText);
		await page.getByRole('button', { name: /1개 답글/ }).click();
		await expect(reply).toBeVisible();

		expect(await reply.evaluate((element) => getComputedStyle(element).userSelect)).toBe('none');
	});
});
