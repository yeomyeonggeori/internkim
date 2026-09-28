import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-mentions';
const reader = { id: 'person-reader', email: 'reader@example.com' };
const members = Array.from({ length: 3 }, (_, index) => ({
	externalID: `external-${index}`,
	name: `박예시${index}`,
	role: 'member'
}));
const mentioned = members[1];

const mentionMessage = {
	id: 'message-mention',
	sender: { id: 'person-author', name: '최견본', email: 'author@example.com' },
	text: `@${mentioned.name} 확인 부탁해요`,
	sentAt: '2026-09-28T01:00:00Z',
	mentions: { externalIDs: [mentioned.externalID], isEveryone: false }
};

test.describe('messenger mentions', () => {
	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: channelID, name: '멘션 채널', kind: 'group', myRole: 'member', members }],
			[mentionMessage]
		);
		await page.goto(`/messenger?channel=${channelID}`);
	});

	test('a mention shows the person on hover and opens their profile on click', async ({ page }) => {
		const chip = page.getByRole('button', { name: `@${mentioned.name}` });
		await chip.hover();
		await expect(page.getByRole('button', { name: '메시지' })).toBeVisible();

		await chip.click();
		const profile = page.getByRole('dialog', { name: '프로필' });
		await expect(profile.getByText(mentioned.name, { exact: true })).toBeVisible();
		await expect(profile.getByRole('button', { name: '메시지' })).toBeVisible();
	});
});
