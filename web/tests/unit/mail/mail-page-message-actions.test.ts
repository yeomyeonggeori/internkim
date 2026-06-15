import { beforeEach, describe, expect, mock, test } from 'bun:test';

import { emptyComposeDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import type { MailPageControllerState } from '../../../src/routes/mail/mail-page-controller-types';
import type { Mailbox, MailMessage } from '../../../src/routes/mail/mail-types';
import { mailText } from '../../../src/routes/mail/text';

type MailMessageListResult = {
	messages: MailMessage[];
	nextCursor: string;
};

type Deferred<Value> = {
	promise: Promise<Value>;
	resolve: (value: Value) => void;
	reject: (error: Error) => void;
};

let mailboxListResponses: Array<Promise<Mailbox[]>>;
let mailMessageListResponses: Array<Promise<MailMessageListResult>>;
let mailMessageDetailResponses: Array<Promise<Partial<MailMessage>>>;
let updateMessageFlagResponses: Array<Promise<void>>;
let moveMessageResponses: Array<Promise<void>>;
let fetchMailboxesCallCount: number;
let fetchMailMessagesCallCount: number;
let fetchMailMessageCallCount: number;
let moveMailMessageCallCount: number;

const fetchMailboxesMock = mock(() => {
	fetchMailboxesCallCount += 1;
	return mailboxListResponses.shift() ?? Promise.resolve([]);
});
const fetchMailMessagesMock = mock(() => {
	fetchMailMessagesCallCount += 1;
	return mailMessageListResponses.shift() ?? Promise.resolve({ messages: [], nextCursor: '' });
});
const fetchMailMessageMock = mock(() => {
	fetchMailMessageCallCount += 1;
	return mailMessageDetailResponses.shift() ?? Promise.resolve({});
});
const updateMailMessageFlagsMock = mock(() => updateMessageFlagResponses.shift() ?? Promise.resolve());
const moveMailMessageMock = mock(() => {
	moveMailMessageCallCount += 1;
	return moveMessageResponses.shift() ?? Promise.resolve();
});

mock.module('../../../src/routes/mail/mail-api', () => ({
	fetchMailboxes: fetchMailboxesMock,
	fetchMailMessage: fetchMailMessageMock,
	fetchMailMessages: fetchMailMessagesMock,
	moveMailMessage: moveMailMessageMock,
	updateMailMessageFlags: updateMailMessageFlagsMock
}));

const { loadMessageDetail, loadMessagesPage, loadPageMailboxes, moveSelectedMailMessage, selectMailPageMailbox, toggleSelectedMailMessageRead } = await import('../../../src/routes/mail/mail-page-message-actions');

describe('mail page message actions', () => {
	beforeEach(() => {
		mailboxListResponses = [];
		mailMessageListResponses = [];
		mailMessageDetailResponses = [];
		updateMessageFlagResponses = [];
		moveMessageResponses = [];
		fetchMailboxesCallCount = 0;
		fetchMailMessagesCallCount = 0;
		fetchMailMessageCallCount = 0;
		moveMailMessageCallCount = 0;
		mock.clearAllMocks();
	});

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

	test('does not reuse stale message detail after replacing the current message list', async () => {
		const staleMessage = { ...inboxMessage, body: 'Stale detail' };
		const freshMessage = { ...inboxMessage, subject: 'Fresh inbox' };
		const controller = createController({
			messages: [inboxMessage],
			selectedMessage: staleMessage,
			messageDetailCache: new Map([['INBOX:1', staleMessage]])
		});
		mailMessageListResponses.push(Promise.resolve({ messages: [freshMessage], nextCursor: '' }));
		mailMessageDetailResponses.push(Promise.resolve({ body: 'Fresh detail' }));

		await loadMessagesPage(controller, mailText.ko, false);

		expect(fetchMailMessageCallCount).toBe(1);
		expect(controller.selectedMessage).toEqual({ ...freshMessage, body: 'Fresh detail' });
		expect(controller.messageDetailCache.get('INBOX:1')).toEqual({ ...freshMessage, body: 'Fresh detail' });
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

		selectMailPageMailbox(controller, 'INBOX');

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
	});

	test('ignores stale message details after selected message changes', async () => {
		const controller = createController({ messages: [inboxMessage, archiveMessage], selectedMessage: inboxMessage });
		const firstResponse = createDeferred<Partial<MailMessage>>();
		const secondResponse = createDeferred<Partial<MailMessage>>();
		mailMessageDetailResponses.push(firstResponse.promise, secondResponse.promise);

		const firstLoad = loadMessageDetail(controller, mailText.ko, inboxMessage);
		controller.selectedMessage = archiveMessage;
		const secondLoad = loadMessageDetail(controller, mailText.ko, archiveMessage);
		secondResponse.resolve({ body: 'Archive detail' });
		await secondLoad;

		firstResponse.resolve({ body: 'Inbox detail' });
		await firstLoad;

		expect(controller.selectedMessage).toEqual({ ...archiveMessage, body: 'Archive detail' });
		expect(controller.isLoadingMessage).toBe(false);
	});

	test('reuses loaded message detail when selecting the same message again', async () => {
		const controller = createController({ messages: [inboxMessage], selectedMessage: inboxMessage });
		mailMessageDetailResponses.push(Promise.resolve({ body: 'Inbox detail' }));

		await loadMessageDetail(controller, mailText.ko, inboxMessage);
		await loadMessageDetail(controller, mailText.ko, { ...inboxMessage, body: 'Inbox detail' });

		expect(fetchMailMessageCallCount).toBe(1);
		expect(controller.selectedMessage).toEqual({ ...inboxMessage, body: 'Inbox detail' });
	});

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

const inboxMessage: MailMessage = {
	uid: 1,
	mailbox: 'INBOX',
	subject: 'Inbox',
	from: 'inbox@example.com',
	date: '',
	preview: '',
	isRead: false
};

const archiveMessage: MailMessage = {
	uid: 2,
	mailbox: 'Archive',
	subject: 'Archive',
	from: 'archive@example.com',
	date: '',
	preview: '',
	isRead: false
};

function createController(overrides: Partial<MailPageControllerState> = {}) {
	let controller: MailPageControllerState;
	controller = {
		account: { ...emptyMailAccount, isConfigured: true, email: 'staff@example.com' },
		accountDraft: { ...emptyMailAccount, email: 'staff@example.com', imapPassword: '', smtpPassword: '' },
		composeDraft: emptyComposeDraft,
		mailboxes: [],
		messages: [],
		messageDetailCache: new Map(),
		selectedMailbox: 'INBOX',
		selectedMessage: null,
		searchText: '',
		activeSearchText: '',
		nextCursor: '',
		hasMoreMessages: false,
		isUnreadOnly: false,
		hasLoadedAccount: true,
		isLoading: false,
		isLoadingMailboxes: false,
		isLoadingMessages: false,
		isLoadingMessage: false,
		isLoadingMore: false,
		isSavingAccount: false,
		isTestingAccount: false,
		isSending: false,
		isSettingsOpen: false,
		isComposeOpen: false,
		errorMessage: '',
		settingsMessage: '',
		composeMessage: '',
		messageListRequestID: 0,
		messageDetailRequestID: 0,
		pageMailboxes: () => [],
		visibleMessages: () => controller.messages,
		mailActorEmail: () => 'staff@example.com',
		mailErrors: (fallback: string) => ({ fallback, serviceUnavailable: 'service unavailable' }),
		resetMessageList: () => {
			controller.messages = [];
			controller.selectedMessage = null;
			controller.nextCursor = '';
			controller.hasMoreMessages = false;
		},
		loadMail: () => Promise.resolve(),
		loadMessages: () => loadMessagesPage(controller, mailText.ko, false),
		...overrides
	};
	return controller;
}

function createDeferred<Value>(): Deferred<Value> {
	let resolve: (value: Value) => void = () => {};
	let reject: (error: Error) => void = () => {};
	const promise = new Promise<Value>((resolveValue, rejectValue) => {
		resolve = resolveValue;
		reject = rejectValue;
	});
	return { promise, resolve, reject };
}
