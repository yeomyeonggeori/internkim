import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	archiveMessage,
	createController,
	createDeferred,
	fetchMailboxesCallCount,
	fetchMailMessagesCallCount,
	inboxMessage,
	loadMessagesPage,
	loadPageMailboxes,
	mailMessageListResponses,
	mailboxListResponses,
	resetMailPageMessageActionTestState,
	selectMailPageMailbox,
	type MailMessageListResult
} from './mail-page-message-actions-test-helpers';
import type { Mailbox } from '../../../src/routes/mail/mail-types';

describe('mail page message list loading actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('ignores stale message list responses after mailbox changes', async () => {
		const controller = createController();
		const firstResponse = createDeferred<MailMessageListResult>();
		const secondResponse = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(firstResponse.promise, secondResponse.promise);

		const firstLoad = loadMessagesPage(controller, mailText.ko, false);
		controller.selectedMailbox = 'Archive';
		const secondLoad = loadMessagesPage(controller, mailText.ko, false);
		secondResponse.resolve({ messages: [archiveMessage], nextCursor: '' });
		await secondLoad;

		firstResponse.resolve({ messages: [inboxMessage], nextCursor: 'older' });
		await firstLoad;

		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.selectedMessage).toEqual(archiveMessage);
		expect(controller.nextCursor).toBe('');
	});

	test('keeps existing messages visible while replacing the current message list', async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		const response = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(response.promise);

		const loadMessages = loadMessagesPage(controller, mailText.ko, false);

		expect(controller.messages).toEqual([inboxMessage]);
		expect(controller.selectedMessage).toEqual(inboxMessage);

		response.resolve({ messages: [archiveMessage], nextCursor: '' });
		await loadMessages;

		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.selectedMessage).toEqual(archiveMessage);
	});

	test('hides previous mailbox messages while loading a mailbox without cached messages', () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		const response = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(response.promise);

		selectMailPageMailbox(controller, mailText.ko, 'Archive');

		expect(controller.selectedMailbox).toBe('Archive');
		expect(controller.messages).toEqual([]);
		expect(controller.selectedMessage).toBe(null);
		expect(controller.isLoadingMessages).toBe(true);
	});

	test('shows message list loading without clearing existing messages', async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		const response = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(response.promise);

		const loadMessages = loadMessagesPage(controller, mailText.ko, false);

		expect(controller.isLoadingMessages).toBe(true);
		expect(controller.messages).toEqual([inboxMessage]);

		response.resolve({ messages: [archiveMessage], nextCursor: '' });
		await loadMessages;

		expect(controller.isLoadingMessages).toBe(false);
		expect(controller.messages).toEqual([archiveMessage]);
	});

	test('keeps existing messages visible when replacing the current message list fails', async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		const response = createDeferred<MailMessageListResult>();
		mailMessageListResponses.push(response.promise);

		const loadMessages = loadMessagesPage(controller, mailText.ko, false);
		response.reject(new Error('imap timeout'));
		await loadMessages;

		expect(controller.messages).toEqual([inboxMessage]);
		expect(controller.selectedMessage).toEqual(inboxMessage);
		expect(controller.isLoadingMessages).toBe(false);
		expect(controller.errorMessage).toBe('imap timeout');
	});

	test('does not reload messages when selecting the active mailbox', () => {
		const controller = createController({ selectedMailbox: 'INBOX' });

		selectMailPageMailbox(controller, mailText.ko, 'INBOX');

		expect(fetchMailMessagesCallCount).toBe(0);
	});

	test('tracks mailbox list loading separately from message loading', async () => {
		const controller = createController();
		const response = createDeferred<Mailbox[]>();
		mailboxListResponses.push(response.promise);

		const loadMailboxes = loadPageMailboxes(controller, mailText.ko);

		expect(controller.isLoadingMailboxes).toBe(true);
		expect(controller.isLoadingMessages).toBe(false);

		response.resolve([{ name: 'INBOX', displayName: 'INBOX', unseen: 0, total: 1 }]);
		await loadMailboxes;

		expect(controller.isLoadingMailboxes).toBe(false);
		expect(controller.mailboxes).toEqual([{ name: 'INBOX', displayName: 'INBOX', unseen: 0, total: 1 }]);
		expect(fetchMailboxesCallCount).toBe(1);
	});
});
