import { expect, test } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const agent = { id: 'person-agent', name: '김인턴', email: 'agent@example.com' };
const agentReplyTimeoutMs = 120_000;

test('the agent working indicator names the agent and gives up when no reply comes', async ({ page }) => {
	await page.clock.install();
	const messages: object[] = [{ id: 'message-earlier', sender: agent, text: '무엇을 도와드릴까요?', sentAt: '2026-09-28T01:00:00Z' }];
	await mockDeviceMessenger(
		page,
		reader,
		[{ id: 'channel-agent', name: '김인턴', kind: 'dm', isWithTheAgent: true }],
		messages
	);
	await page.route('**/agent/api/dm**', async (route) => {
		if (route.request().method() === 'POST') {
			const { message } = route.request().postDataJSON() as { message: string };
			messages.push({ id: `message-${messages.length}`, sender: reader, text: message, sentAt: '2026-09-28T01:05:00Z' });
		}
		await route.fulfill({
			json: {
				conversationID: 'channel-agent',
				currentUserId: reader.id,
				messages,
				hasMoreBefore: false,
				historyCursor: ''
			}
		});
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/messenger?channel=channel-agent');

	await page.getByRole('combobox', { name: '메시지를 입력하세요' }).fill('다음 주 일정 정리해줘');
	await page.getByRole('button', { name: '보내기' }).click();

	const indicator = page.getByRole('status').filter({ hasText: '김인턴이 작업 중이에요' });
	await expect(indicator).toBeVisible();
	const sentMessage = page.getByText('다음 주 일정 정리해줘');
	const sentWhileWorking = await sentMessage.boundingBox();

	await page.clock.fastForward(agentReplyTimeoutMs);
	await expect(indicator).toBeHidden();
	expect(await sentMessage.boundingBox()).toEqual(sentWhileWorking);

	messages.push({ id: 'message-late', sender: agent, text: '늦은 답장이에요', sentAt: '2026-09-28T01:10:00Z' });
	await page.clock.fastForward(30_000);
	await expect(page.locator('[data-message-id="message-late"]')).toBeVisible();
});
