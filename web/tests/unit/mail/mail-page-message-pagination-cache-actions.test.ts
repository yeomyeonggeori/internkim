import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	createController,
	fetchMailMessageCallCount,
	fetchMailMessagesCallCount,
	inboxMessage,
	loadMessagesPage,
	mailMessageListResponses,
	messagePageCacheKey,
	resetMailPageMessageActionTestState
} from './mail-page-message-actions-test-helpers';

describe('mail page message pagination cache actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('uses a cached next page without fetching messages', async () => {
		const olderInboxMessage = { ...inboxMessage, uid: 3, subject: 'Older inbox' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			nextCursor: 'next-page',
			hasMoreMessages: true,
			messageListCache: new Map([
				[
					messagePageCacheKey('staff@example.com', 'INBOX', '', 1),
					{
						actorEmail: 'staff@example.com',
						mailbox: 'INBOX',
						searchText: '',
						pageIndex: 1,
						cursor: 'next-page',
						messages: [olderInboxMessage],
						nextCursor: '',
						fetchedAt: Date.now()
					}
				]
			])
		});

		await loadMessagesPage(controller, mailText.ko, { mode: 'cache-first', pageIndex: 1 });

		expect(fetchMailMessagesCallCount).toBe(0);
		expect(controller.messagePageIndex).toBe(1);
		expect(controller.messages).toEqual([olderInboxMessage]);
	});

	test('appends the next page onto the messages already shown', async () => {
		const olderInboxMessage = { ...inboxMessage, uid: 3, subject: 'Older inbox' };
		const controller = createController({
			messages: [inboxMessage],
			nextCursor: 'next-page',
			hasMoreMessages: true,
			messageListCache: new Map([
				[
					messagePageCacheKey('staff@example.com', 'INBOX', '', 0),
					{
						actorEmail: 'staff@example.com',
						mailbox: 'INBOX',
						searchText: '',
						pageIndex: 0,
						cursor: '',
						messages: [inboxMessage],
						nextCursor: 'next-page',
						fetchedAt: Date.now()
					}
				]
			])
		});
		mailMessageListResponses.push(Promise.resolve({ messages: [olderInboxMessage], nextCursor: '' }));

		await loadMessagesPage(controller, mailText.ko, { mode: 'cache-first', pageIndex: 1 });

		expect(controller.messages).toEqual([inboxMessage, olderInboxMessage]);
		expect(controller.hasMoreMessages).toBe(false);
	});

	test('keeps cached message detail while refreshing the list envelope', async () => {
		const cachedMessage = { ...inboxMessage, body: 'Cached detail' };
		const freshMessage = { ...inboxMessage, subject: 'Fresh inbox' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: cachedMessage,
			messageDetailCache: new Map([['INBOX:1', cachedMessage]])
		});
		mailMessageListResponses.push(Promise.resolve({ messages: [freshMessage], nextCursor: '' }));

		await loadMessagesPage(controller, mailText.ko, false);

		expect(fetchMailMessageCallCount).toBe(0);
		expect(controller.selectedMessage).toEqual({ ...freshMessage, body: 'Cached detail' });
		expect(controller.messageDetailCache.get('INBOX:1')).toEqual({ ...freshMessage, body: 'Cached detail' });
	});
});
