import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const author = { id: 'person-author', name: '박예시', email: 'author@example.com' };

test('an edited message says so beside its time, and an unedited one does not', async ({ page }) => {
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: 'channel-edit', name: '수정 채널', kind: 'group', myRole: 'member' }],
		[
			{ id: 'message-plain', sender: author, text: '그대로인 문장', sentAt: '2026-09-28T01:00:00Z' },
			{
				id: 'message-edited',
				sender: author,
				text: '고친 문장',
				sentAt: '2026-09-28T01:05:00Z',
				editedAt: '2026-09-28T01:06:00Z'
			}
		]
	);
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-edit');

	await expect(page.locator('[data-message-id="message-edited"] time')).toContainText('수정됨');
	await expect(page.locator('[data-message-id="message-plain"] time')).not.toContainText('수정됨');
});
