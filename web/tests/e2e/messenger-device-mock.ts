import type { Page } from '@playwright/test';
import { mockBuzzDisabled } from './buzz-test-routes';

export type MockReader = { id: string; email: string };

export type MockConversation = {
	id: string;
	name: string;
	kind: 'dm' | 'group';
	myRole?: string;
	unreadCount?: number;
};

export async function mockDeviceMessenger(
	page: Page,
	reader: MockReader,
	conversations: MockConversation[],
	messages: object[]
): Promise<void> {
	await mockBuzzDisabled(page);
	await page.route('**/auth/session**', async (route) => {
		await route.fulfill({ json: { authenticated: true, email: reader.email } });
	});
	await page.route('**/admin/api/session', async (route) => {
		await route.fulfill({ json: { email: reader.email } });
	});
	await page.route('**/admin/api/locale', async (route) => {
		await route.fulfill({ json: { locale: 'ko' } });
	});
	await page.route('**/agent/api/channels', async (route) => {
		await route.fulfill({ json: { conversations } });
	});
	await page.route('**/agent/api/people', async (route) => {
		await route.fulfill({ json: { people: [] } });
	});
	await page.route('**/agent/api/dm**', async (route) => {
		await route.fulfill({
			json: {
				conversationID: conversations[0]?.id ?? '',
				currentUserId: reader.id,
				messages,
				hasMoreBefore: false,
				historyCursor: ''
			}
		});
	});
}
