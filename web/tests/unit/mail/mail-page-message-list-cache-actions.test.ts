import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	archiveMessage,
	createController,
	createDeferred,
	fetchMailMessagesCallCount,
	inboxMessage,
	legacyMessagePageCacheKey,
	loadMessagesPage,
	mailMessageListResponses,
	messageKey,
	messagePageCacheKey,
	resetMailPageMessageActionTestState,
	selectMailPageMailbox,
	type MailMessageListResult
} from './mail-page-message-actions-test-helpers';

describe('mail page message list cache actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('uses a fresh cached mailbox page without fetching messages', async () => {
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			messageListCache: new Map([
				[
					messagePageCacheKey('member@example.com', 'Archive', '', 0),
					{
						actorEmail: 'member@example.com',
						mailbox: 'Archive',
						searchText: '',
						pageIndex: 0,
						cursor: '',
						messages: [archiveMessage],
						nextCursor: '',
						fetchedAt: Date.now()
					}
				]
			]),
			messageDetailCache: new Map([[messageKey(archiveMessage), { ...archiveMessage, body: 'Archive detail' }]])
		});

		await selectMailPageMailbox(controller, mailText.ko, 'Archive');

		expect(fetchMailMessagesCallCount).toBe(0);
		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.selectedMessage).toEqual({ ...archiveMessage, body: 'Archive detail' });
	});

	test('does not reuse a legacy page cache entry from another actor', async () => {
		const otherAccountMessage = { ...archiveMessage, subject: 'Other account archive' };
		const currentAccountMessage = { ...archiveMessage, subject: 'Current account archive' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			messageListCache: new Map([
				[
					legacyMessagePageCacheKey('Archive', '', 0),
					{
						actorEmail: 'other@example.com',
						mailbox: 'Archive',
						searchText: '',
						pageIndex: 0,
						cursor: '',
						messages: [otherAccountMessage],
						nextCursor: '',
						fetchedAt: Date.now()
					}
				]
			])
		});
		mailMessageListResponses.push(Promise.resolve({ messages: [currentAccountMessage], nextCursor: '' }));

		await selectMailPageMailbox(controller, mailText.ko, 'Archive');

		expect(fetchMailMessagesCallCount).toBe(1);
		expect(controller.messages).toEqual([currentAccountMessage]);
	});

	test('shows stale cached mailbox messages before refreshing them', async () => {
		const refreshedMessage = { ...archiveMessage, subject: 'Refreshed archive' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			messageListCache: new Map([
				[
					messagePageCacheKey('member@example.com', 'Archive', '', 0),
					{
						actorEmail: 'member@example.com',
						mailbox: 'Archive',
						searchText: '',
						pageIndex: 0,
						cursor: '',
						messages: [archiveMessage],
						nextCursor: '',
						fetchedAt: Date.now() - 31_000
					}
				]
			]),
			messageDetailCache: new Map([[messageKey(archiveMessage), { ...archiveMessage, body: 'Archive detail' }]])
		});
		const response = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(response.promise);

		const selectMailbox = selectMailPageMailbox(controller, mailText.ko, 'Archive');
		await Promise.resolve();
		await Promise.resolve();

		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.isLoadingMessages).toBe(true);

		response.resolve({ messages: [refreshedMessage], nextCursor: '' });
		await selectMailbox;

		expect(fetchMailMessagesCallCount).toBe(1);
		expect(controller.messages).toEqual([refreshedMessage]);
		expect(controller.isLoadingMessages).toBe(false);
	});

	test('bypasses a fresh page cache for a forced refresh', async () => {
		const refreshedMessage = { ...inboxMessage, subject: 'Forced refresh' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			messageListCache: new Map([
				[
					messagePageCacheKey('member@example.com', 'INBOX', '', 0),
					{
						actorEmail: 'member@example.com',
						mailbox: 'INBOX',
						searchText: '',
						pageIndex: 0,
						cursor: '',
						messages: [inboxMessage],
						nextCursor: '',
						fetchedAt: Date.now()
					}
				]
			])
		});
		mailMessageListResponses.push(Promise.resolve({ messages: [refreshedMessage], nextCursor: '' }));

		await loadMessagesPage(controller, mailText.ko, { mode: 'force', pageIndex: 0 });

		expect(fetchMailMessagesCallCount).toBe(1);
		expect(controller.messages).toEqual([refreshedMessage]);
	});
});
