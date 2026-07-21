import { beforeEach, describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';
import {
	archiveMessage,
	createController,
	createDeferred,
	fetchMailboxesCallCount,
	fetchMailMessageCallCount,
	fetchMailMessagesCallCount,
	inboxMessage,
	loadMessageDetail,
	mailboxListResponses,
	moveMailMessageCallCount,
	moveMessageResponses,
	moveSelectedMailMessage,
	resetMailPageMessageActionTestState,
	toggleSelectedMailMessageRead,
	updateMessageFlagResponses
} from './mail-page-message-actions-test-helpers';

describe('mail page message mutation actions', () => {
	beforeEach(resetMailPageMessageActionTestState);

	test('does not overwrite the current selected message after a stale read toggle', async () => {
		const controller = createController({ messages: [inboxMessage, archiveMessage], selectedMessage: inboxMessage });
		const updateResponse = createDeferred<void>();
		updateMessageFlagResponses.push(updateResponse.promise);

		const updateReadState = toggleSelectedMailMessageRead(controller, mailText.ko);
		controller.selectedMessage = archiveMessage;
		updateResponse.resolve();
		await updateReadState;

		expect(controller.selectedMessage).toEqual(archiveMessage);
		expect(controller.messages).toEqual([{ ...inboxMessage, isRead: true }, archiveMessage]);
	});

	test('updates cached message detail when toggling read state', async () => {
		const detailedMessage = { ...inboxMessage, body: 'Inbox detail' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: detailedMessage,
			messageDetailCache: new Map([['INBOX:1', detailedMessage]])
		});
		updateMessageFlagResponses.push(Promise.resolve());

		await toggleSelectedMailMessageRead(controller, mailText.ko);
		await loadMessageDetail(controller, mailText.ko, inboxMessage);

		expect(fetchMailMessageCallCount).toBe(0);
		expect(controller.selectedMessage).toEqual({ ...detailedMessage, isRead: true });
		expect(controller.messageDetailCache.get('INBOX:1')).toEqual({ ...detailedMessage, isRead: true });
	});

	test('refreshes mailbox counts after toggling read state', async () => {
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: inboxMessage,
			mailboxes: [{ name: 'INBOX', displayName: 'INBOX', unseen: 1, total: 3 }]
		});
		updateMessageFlagResponses.push(Promise.resolve());
		mailboxListResponses.push(Promise.resolve([{ name: 'INBOX', displayName: 'INBOX', unseen: 0, total: 3 }]));

		await toggleSelectedMailMessageRead(controller, mailText.ko);

		expect(fetchMailboxesCallCount).toBe(1);
		expect(controller.mailboxes).toEqual([{ name: 'INBOX', displayName: 'INBOX', unseen: 0, total: 3 }]);
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
