import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	archiveMessage,
	createController,
	createDeferred,
	fetchMailboxesCallCount,
	fetchMailMessagesCallCount,
	inboxMessage,
	mailboxListResponses,
	moveMailMessageCallCount,
	moveMessageResponses,
	moveSelectedMailMessage,
	resetMailPageMessageActionTestState,
	selectMailPageMessage,
	updateMailMessageFlagsCallCount
} from './mail-page-message-actions-test-helpers';

describe('mail page message mutation actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('marks an unread message read when it is opened', async () => {
		const unreadMessage = { ...inboxMessage, isRead: false };
		const controller = createController({ messages: [unreadMessage] });
		mailboxListResponses.push(Promise.resolve([{ name: 'INBOX', displayName: 'INBOX', unseen: 0, total: 3 }]));

		selectMailPageMessage(controller, mailText.ko, unreadMessage);
		await Promise.resolve();
		await Promise.resolve();

		expect(updateMailMessageFlagsCallCount).toBe(1);
		expect(controller.messages).toEqual([{ ...unreadMessage, isRead: true }]);
	});

	test('does not re-mark a message that is already read', async () => {
		const readMessage = { ...inboxMessage, isRead: true };
		const controller = createController({ messages: [readMessage] });

		selectMailPageMessage(controller, mailText.ko, readMessage);
		await Promise.resolve();

		expect(updateMailMessageFlagsCallCount).toBe(0);
	});

	test('removes moved messages locally and refreshes mailbox counts', async () => {
		const controller = createController({
			messages: [inboxMessage, archiveMessage],
			selectedMessage: inboxMessage,
			mailboxes: [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }],
			pageMailboxes: () => [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }]
		});
		moveMessageResponses.push(Promise.resolve());
		mailboxListResponses.push(Promise.resolve([{ name: 'Archive', displayName: 'Archive', unseen: 1, total: 2 }]));

		await moveSelectedMailMessage(controller, mailText.ko, 'archive');

		expect(moveMailMessageCallCount).toBe(1);
		expect(fetchMailMessagesCallCount).toBe(0);
		expect(fetchMailboxesCallCount).toBe(1);
		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.selectedMessage).toEqual(archiveMessage);
		expect(controller.mailboxes).toEqual([{ name: 'Archive', displayName: 'Archive', unseen: 1, total: 2 }]);
	});

	test('removes the moved message when selection changes while moving it', async () => {
		const controller = createController({
			messages: [inboxMessage, archiveMessage],
			selectedMessage: inboxMessage,
			mailboxes: [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }],
			pageMailboxes: () => [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }]
		});
		const moveResponse = createDeferred<void>();
		moveMessageResponses.push(moveResponse.promise);

		const moveMessage = moveSelectedMailMessage(controller, mailText.ko, 'archive');
		controller.selectedMessage = archiveMessage;
		moveResponse.resolve();
		await moveMessage;

		expect(controller.messages).toEqual([archiveMessage]);
		expect(controller.selectedMessage).toEqual(archiveMessage);
	});

	test('clears previous errors after moving a message succeeds', async () => {
		const controller = createController({
			errorMessage: 'previous failure',
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			mailboxes: [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }],
			pageMailboxes: () => [{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 }]
		});
		moveMessageResponses.push(Promise.resolve());

		await moveSelectedMailMessage(controller, mailText.ko, 'archive');

		expect(controller.errorMessage).toBe('');
	});
});
