import { emptyComposeDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import type { MailPageControllerState } from '../../../src/routes/mail/mail-page-controller-types';
import type { MailMessage } from '../../../src/routes/mail/mail-types';
import { mailText } from '../../../src/routes/mail/text';
import { resetMailApiTestMockState } from './mail-api-test-mock';
export type { MailMessageListResult } from './mail-api-test-mock';
export {
	fetchMailboxesCallCount,
	fetchMailMessageCallCount,
	fetchMailMessagesCallCount,
	mailboxListResponses,
	mailMessageDetailResponses,
	mailMessageListResponses,
	moveMailMessageCallCount,
	moveMessageResponses,
	updateMailMessageFlagsCallCount,
	updateMessageFlagResponses
} from './mail-api-test-mock';

export type Deferred<Value> = {
	promise: Promise<Value>;
	resolve: (value: Value) => void;
	reject: (error: Error) => void;
};

export const {
	loadMessageDetail,
	loadMessagesPage,
	loadPageMailboxes,
	markMailMessageRead,
	moveSelectedMailMessage,
	selectMailPageMessage,
	selectMailPageMailbox
} = await import('../../../src/routes/mail/mail-page-message-actions');

export function resetMailPageMessageActionTestState() {
	resetMailApiTestMockState();
}

export const inboxMessage: MailMessage = {
	uid: 1,
	mailbox: 'INBOX',
	subject: 'Inbox',
	from: 'inbox@example.com',
	date: '',
	preview: '',
	isRead: false
};

export const archiveMessage: MailMessage = {
	uid: 2,
	mailbox: 'Archive',
	subject: 'Archive',
	from: 'archive@example.com',
	date: '',
	preview: '',
	isRead: false
};

export function createController(overrides: Partial<MailPageControllerState> = {}) {
	let controller: MailPageControllerState;
	controller = {
		account: { ...emptyMailAccount, isConfigured: true, email: 'staff@example.com' },
		accountDraft: { ...emptyMailAccount, email: 'staff@example.com', imapPassword: '', smtpPassword: '' },
		composeDraft: emptyComposeDraft,
		mailboxes: [],
		messages: [],
		messageListCache: new Map(),
		messageDetailCache: new Map(),
		selectedMailbox: 'INBOX',
		selectedMessage: null,
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
		isSettingsOpen: false,
		isComposeOpen: false,
		composeFocusField: 'to',
		errorMessage: '',
		settingsMessage: '',
		composeMessage: '',
		messageListRequestID: 0,
		messageDetailRequestID: 0,
		pageMailboxes: () => [],
		visibleMessages: () => controller.messages,
		canLoadMoreMessages: () => controller.hasMoreMessages,
		mailActorEmail: () => 'staff@example.com',
		mailErrors: (fallback: string) => ({ fallback, serviceUnavailable: 'service unavailable' }),
		resetMessageList: () => {
			controller.messages = [];
			controller.messageListCache.clear();
			controller.messageDetailCache.clear();
			controller.selectedMessage = null;
			controller.activeSearchText = '';
			controller.messagePageIndex = 0;
			controller.nextCursor = '';
			controller.hasMoreMessages = false;
		},
		loadMail: () => Promise.resolve(),
		loadMessages: () => loadMessagesPage(controller, mailText.ko, false),
		loadMoreMessages: () => loadMessagesPage(controller, mailText.ko, { mode: 'cache-first', pageIndex: controller.messagePageIndex + 1 }),
		setUnreadOnly: (isUnreadOnly: boolean) => {
			controller.isUnreadOnly = isUnreadOnly;
			return Promise.resolve();
		},
		...overrides
	};
	return controller;
}

export function createDeferred<Value>(): Deferred<Value> {
	let resolve: (value: Value) => void = () => {};
	let reject: (error: Error) => void = () => {};
	const promise = new Promise<Value>((resolveValue, rejectValue) => {
		resolve = resolveValue;
		reject = rejectValue;
	});
	return { promise, resolve, reject };
}

export function messagePageCacheKey(actorEmail: string, mailbox: string, searchText: string, pageIndex: number) {
	return `${actorEmail}\u0000${mailbox}\u0000${searchText}\u0000${pageIndex}`;
}

export function legacyMessagePageCacheKey(mailbox: string, searchText: string, pageIndex: number) {
	return `${mailbox}\u0000${searchText}\u0000${pageIndex}`;
}

export function messageKey(message: MailMessage) {
	return `${message.mailbox}:${message.uid}`;
}
