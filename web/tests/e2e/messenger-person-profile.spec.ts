import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };
const counterpart = { id: 'person-counterpart', name: '박예시', email: 'counterpart@example.com', externalID: 'external-counterpart' };

test.describe('messenger person profile', () => {
	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await mockDeviceMessenger(
			page,
			reader,
			[{ id: 'channel-dm', name: counterpart.name, kind: 'dm', counterpart: { externalID: counterpart.externalID } }],
			[{ id: 'message-from-counterpart', sender: counterpart, text: '안녕하세요.', sentAt: '2026-10-12T01:00:00Z' }]
		);
		await page.goto('/messenger?channel=channel-dm');
		await expect(page.locator('[data-message-id="message-from-counterpart"]')).toBeVisible();
	});

	test('the direct message header opens the counterpart profile', async ({ page }) => {
		await page.locator('header').getByRole('button', { name: '프로필' }).click();
		const profile = page.getByRole('dialog', { name: '프로필' });
		await expect(profile.getByText(counterpart.name, { exact: true })).toBeVisible();
		await expect(profile.getByRole('button', { name: '메시지' })).toBeVisible();
	});

	test('the sender picture opens the sender profile', async ({ page }) => {
		await page.locator('[data-message-id="message-from-counterpart"] [data-slot="sender-avatar"]').click();
		const profile = page.getByRole('dialog', { name: '프로필' });
		await expect(profile.getByText(counterpart.name, { exact: true })).toBeVisible();
	});
});
