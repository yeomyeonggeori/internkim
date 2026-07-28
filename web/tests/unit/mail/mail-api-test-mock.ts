import { mock } from 'bun:test';

import { emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import type { MailAccount, Mailbox, MailMessage } from '../../../src/routes/mail/mail-types';

export type MailMessageListResult = {
	messages: MailMessage[];
	nextCursor: string;
};

export type MailBootstrapResult = {
	account: MailAccount;
	mailboxes: Mailbox[];
	messages: MailMessage[];
	nextCursor: string;
	hasCachedMailboxes: boolean;
	hasCachedMessages: boolean;
};

export let fetchMailAccountResponses: Array<Promise<MailAccount>>;
export let fetchMailBootstrapResponses: Array<Promise<MailBootstrapResult>>;
export let mailboxListResponses: Array<Promise<Mailbox[]>>;
export let mailMessageListResponses: Array<Promise<MailMessageListResult>>;
export let mailMessageDetailResponses: Array<Promise<Partial<MailMessage>>>;
export let saveMailAccountResponses: Array<Promise<MailAccount>>;
export let sendMailMessageResponses: Array<Promise<void>>;
export let testMailAccountResponses: Array<Promise<void>>;
export let updateMessageFlagResponses: Array<Promise<void>>;
export let moveMessageResponses: Array<Promise<void>>;
export let fetchMailboxesCallCount: number;
export let fetchMailMessagesCallCount: number;
export let fetchMailMessageCallCount: number;
export let saveMailAccountCallCount: number;
export let moveMailMessageCallCount: number;
export let updateMailMessageFlagsCallCount: number;
export let savedMailAccountActorEmail: string;

const defaultBootstrapResult: MailBootstrapResult = {
	account: emptyMailAccount,
	mailboxes: [],
	messages: [],
	nextCursor: '',
	hasCachedMailboxes: false,
	hasCachedMessages: false
};

const fetchMailAccountMock = mock(() => fetchMailAccountResponses.shift() ?? Promise.resolve(emptyMailAccount));
const fetchMailBootstrapMock = mock(() => fetchMailBootstrapResponses.shift() ?? Promise.resolve(defaultBootstrapResult));
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
const saveMailAccountMock = mock((actorEmail: string) => {
	saveMailAccountCallCount += 1;
	savedMailAccountActorEmail = actorEmail;
	return saveMailAccountResponses.shift() ?? Promise.resolve(emptyMailAccount);
});
const sendMailMessageMock = mock(() => sendMailMessageResponses.shift() ?? Promise.resolve());
const testMailAccountMock = mock(() => testMailAccountResponses.shift() ?? Promise.resolve());
const updateMailMessageFlagsMock = mock(() => {
	updateMailMessageFlagsCallCount += 1;
	return updateMessageFlagResponses.shift() ?? Promise.resolve();
});
const moveMailMessageMock = mock(() => {
	moveMailMessageCallCount += 1;
	return moveMessageResponses.shift() ?? Promise.resolve();
});

mock.module('../../../src/routes/mail/mail-api', () => ({
	fetchMailAccount: fetchMailAccountMock,
	fetchMailBootstrap: fetchMailBootstrapMock,
	fetchMailboxes: fetchMailboxesMock,
	fetchMailMessage: fetchMailMessageMock,
	fetchMailMessages: fetchMailMessagesMock,
	moveMailMessage: moveMailMessageMock,
	saveMailAccount: saveMailAccountMock,
	sendMailMessage: sendMailMessageMock,
	testMailAccount: testMailAccountMock,
	updateMailMessageFlags: updateMailMessageFlagsMock
}));

export function resetMailApiTestMockState() {
	fetchMailAccountResponses = [];
	fetchMailBootstrapResponses = [];
	mailboxListResponses = [];
	mailMessageListResponses = [];
	mailMessageDetailResponses = [];
	saveMailAccountResponses = [];
	sendMailMessageResponses = [];
	testMailAccountResponses = [];
	updateMessageFlagResponses = [];
	updateMailMessageFlagsCallCount = 0;
	moveMessageResponses = [];
	fetchMailboxesCallCount = 0;
	fetchMailMessagesCallCount = 0;
	fetchMailMessageCallCount = 0;
	saveMailAccountCallCount = 0;
	moveMailMessageCallCount = 0;
	savedMailAccountActorEmail = '';
	mock.clearAllMocks();
}

resetMailApiTestMockState();
