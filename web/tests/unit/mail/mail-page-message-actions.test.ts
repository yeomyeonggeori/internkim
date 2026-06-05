import { beforeEach, describe, expect, mock, test } from 'bun:test';

import { emptyComposeDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import type { MailPageControllerState } from '../../../src/routes/mail/mail-page-controller-types';
import type { MailMessage } from '../../../src/routes/mail/mail-types';
import { mailText } from '../../../src/routes/mail/text';

type MailMessageListResult = {
	messages: MailMessage[];
	nextCursor: string;
};

type Deferred<Value> = {
	promise: Promise<Value>;
	resolve: (value: Value) => void;
};

let mailMessageListResponses: Array<Promise<MailMessageListResult>>;
let mailMessageDetailResponses: Array<Promise<Partial<MailMessage>>>;
let updateMessageFlagResponses: Array<Promise<void>>;

const fetchMailMessagesMock = mock(() => mailMessageListResponses.shift() ?? Promise.resolve({ messages: [], nextCursor: '' }));
const fetchMailMessageMock = mock(() => mailMessageDetailResponses.shift() ?? Promise.resolve({}));
const updateMailMessageFlagsMock = mock(() => updateMessageFlagResponses.shift() ?? Promise.resolve());

mock.module('../../../src/routes/mail/mail-api', () => ({
	fetchMailboxes: mock(() => Promise.resolve([])),
	fetchMailMessage: fetchMailMessageMock,
	fetchMailMessages: fetchMailMessagesMock,
	moveMailMessage: mock(() => Promise.resolve()),
	updateMailMessageFlags: updateMailMessageFlagsMock
}));

const { loadMessageDetail, loadMessagesPage, toggleSelectedMailMessageRead } = await import('../../../src/routes/mail/mail-page-message-actions');

describe('mail page message actions', () => {
	beforeEach(() => {
		mailMessageListResponses = [];
		mailMessageDetailResponses = [];
		updateMessageFlagResponses = [];
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
		selectedMailbox: 'INBOX',
		selectedMessage: null,
		searchText: '',
		activeSearchText: '',
		nextCursor: '',
		hasMoreMessages: false,
		isUnreadOnly: false,
		isLoading: false,
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
	const promise = new Promise<Value>((resolveValue) => {
		resolve = resolveValue;
	});
	return { promise, resolve };
}
