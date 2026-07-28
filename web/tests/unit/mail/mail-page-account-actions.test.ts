import { beforeEach, describe, expect, test } from 'bun:test';

import { emptyComposeDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import type { MailPageControllerState } from '../../../src/routes/mail/mail-page-controller-types';
import type { MailAccount, MailMessage } from '../../../src/routes/mail/mail-types';
import { mailText } from '../../../src/routes/mail/text';
import {
	resetMailApiTestMockState,
	saveMailAccountCallCount,
	saveMailAccountResponses,
	savedMailAccountActorEmail
} from './mail-api-test-mock';

let loadMailCallCount: number;
let resetMessageListCallCount: number;

const { saveMailAccountDraft } = await import('../../../src/routes/mail/mail-page-account-actions');

describe('mail page account actions', () => {
	beforeEach(() => {
		resetMailApiTestMockState();
		saveMailAccountResponses.push(Promise.resolve({ ...emptyMailAccount, email: 'new@example.com', defaultMailbox: 'INBOX', isConfigured: true }));
		loadMailCallCount = 0;
		resetMessageListCallCount = 0;
	});

	test('clears client mail caches before reloading a saved account', async () => {
		const controller = createController();

		await saveMailAccountDraft(controller, mailText.ko);

		expect(saveMailAccountCallCount).toBe(1);
		expect(savedMailAccountActorEmail).toBe('new@example.com');
		expect(resetMessageListCallCount).toBe(1);
		expect(controller.messages).toEqual([]);
		expect(controller.messageListCache.size).toBe(0);
		expect(controller.messageDetailCache.size).toBe(0);
		expect(controller.mailboxes).toEqual([]);
		expect(controller.selectedMailbox).toBe('INBOX');
		expect(loadMailCallCount).toBe(1);
		expect(controller.isSettingsOpen).toBe(false);
	});
});

const cachedMessage: MailMessage = {
	uid: 1,
	mailbox: 'INBOX',
	subject: 'Cached',
	from: 'old@example.com',
	date: '',
	preview: '',
	isRead: false
};

function createController() {
	let controller: MailPageControllerState;
	controller = {
		account: { ...emptyMailAccount, email: 'old@example.com', isConfigured: true },
		accountDraft: {
			...emptyMailAccount,
			email: 'new@example.com',
			displayName: 'New',
			imapHost: 'imap.example.com',
			imapUsername: 'new@example.com',
			imapPassword: 'imap-secret',
			smtpHost: 'smtp.example.com',
			smtpUsername: 'new@example.com',
			smtpPassword: 'smtp-secret'
		},
		composeDraft: emptyComposeDraft,
		mailboxes: [{ name: 'INBOX', displayName: 'INBOX', unseen: 1, total: 1 }],
		messages: [cachedMessage],
		messageListCache: new Map([
			[
				'old@example.com\u0000INBOX\u0000\u00000',
				{
					actorEmail: 'old@example.com',
					mailbox: 'INBOX',
					searchText: '',
					pageIndex: 0,
					cursor: '',
					messages: [cachedMessage],
					nextCursor: '',
					fetchedAt: Date.now()
				}
			]
		]),
		messageDetailCache: new Map([['INBOX:1', cachedMessage]]),
		selectedMailbox: 'INBOX',
		selectedMessage: cachedMessage,
		searchText: '',
		activeSearchText: '',
		messagePageIndex: 0,
		nextCursor: '',
		hasMoreMessages: false,
		isUnreadOnly: false,
		canSelectFirstMessage: true,
		hasLoadedAccount: true,
		isLoading: false,
		isSyncing: false,
		isLoadingMailboxes: false,
		isLoadingMessages: false,
		isLoadingMessage: false,
		isSavingAccount: false,
		isTestingAccount: false,
		isSending: false,
		isSettingsOpen: true,
		isComposeOpen: false,
		composeFocusField: 'to',
		errorMessage: '',
		settingsMessage: '',
		composeMessage: '',
		messageListRequestID: 0,
		messageDetailRequestID: 0,
		pageMailboxes: () => controller.mailboxes,
		visibleMessages: () => controller.messages,
		canLoadMoreMessages: () => false,
		mailActorEmail: () => controller.account.email,
		mailErrors: (fallback: string) => ({ fallback, serviceUnavailable: 'service unavailable' }),
		resetMessageList: () => {
			resetMessageListCallCount += 1;
			controller.messages = [];
			controller.messageListCache.clear();
			controller.messageDetailCache.clear();
			controller.selectedMessage = null;
			controller.activeSearchText = '';
			controller.messagePageIndex = 0;
			controller.nextCursor = '';
			controller.hasMoreMessages = false;
		},
		loadMail: () => {
			loadMailCallCount += 1;
			return Promise.resolve();
		},
		loadMessages: () => Promise.resolve(),
		loadMoreMessages: () => Promise.resolve(),
		setUnreadOnly: () => Promise.resolve()
	};
	return controller;
}
