import type { MailAccount, MailBootstrap, Mailbox, MailMessage } from './mail-types';

export function normalizeMailAccountResponse(value: unknown): Partial<MailAccount> {
	const objectValue = objectRecord(value);
	if (!objectValue) return {};
	const account: Partial<MailAccount> = {};
	const email = stringField(objectValue, 'email');
	if (email !== undefined) account.email = email;
	const fromAddress = stringField(objectValue, 'fromAddress');
	if (fromAddress !== undefined) account.fromAddress = fromAddress;
	const displayName = stringField(objectValue, 'displayName');
	if (displayName !== undefined) account.displayName = displayName;
	const imapHost = stringField(objectValue, 'imapHost');
	if (imapHost !== undefined) account.imapHost = imapHost;
	const imapPort = numberField(objectValue, 'imapPort');
	if (imapPort !== undefined) account.imapPort = imapPort;
	const imapSecurity = stringField(objectValue, 'imapSecurity');
	if (imapSecurity !== undefined) account.imapSecurity = imapSecurity;
	const imapUsername = stringField(objectValue, 'imapUsername');
	if (imapUsername !== undefined) account.imapUsername = imapUsername;
	const smtpHost = stringField(objectValue, 'smtpHost');
	if (smtpHost !== undefined) account.smtpHost = smtpHost;
	const smtpPort = numberField(objectValue, 'smtpPort');
	if (smtpPort !== undefined) account.smtpPort = smtpPort;
	const smtpSecurity = stringField(objectValue, 'smtpSecurity');
	if (smtpSecurity !== undefined) account.smtpSecurity = smtpSecurity;
	const smtpUsername = stringField(objectValue, 'smtpUsername');
	if (smtpUsername !== undefined) account.smtpUsername = smtpUsername;
	const defaultMailbox = stringField(objectValue, 'defaultMailbox');
	if (defaultMailbox !== undefined) account.defaultMailbox = defaultMailbox;
	const sentMailbox = stringField(objectValue, 'sentMailbox');
	if (sentMailbox !== undefined) account.sentMailbox = sentMailbox;
	const isConfigured = booleanField(objectValue, 'isConfigured');
	if (isConfigured !== undefined) account.isConfigured = isConfigured;
	const hasIMAPPassword = booleanField(objectValue, 'hasIMAPPassword');
	if (hasIMAPPassword !== undefined) account.hasIMAPPassword = hasIMAPPassword;
	const hasSMTPPassword = booleanField(objectValue, 'hasSMTPPassword');
	if (hasSMTPPassword !== undefined) account.hasSMTPPassword = hasSMTPPassword;
	return account;
}

export function normalizeMailboxesResponse(value: unknown): Mailbox[] {
	const objectValue = objectRecord(value);
	const mailboxes = objectValue ? objectValue.mailboxes : undefined;
	if (!Array.isArray(mailboxes)) return [];
	return mailboxes.map(normalizeMailboxResponse).filter(isDefined);
}

export function normalizeMailMessagesResponse(value: unknown) {
	const objectValue = objectRecord(value);
	if (!objectValue) return { messages: [], nextCursor: '' };
	const messages = Array.isArray(objectValue.messages) ? objectValue.messages.map(normalizeMailMessageResponse).filter(isDefined) : [];
	return {
		messages,
		nextCursor: stringField(objectValue, 'nextCursor') ?? ''
	};
}

export function normalizeMailBootstrapResponse(value: unknown): MailBootstrap {
	const objectValue = objectRecord(value);
	if (!objectValue) {
		return emptyMailBootstrap();
	}
	return {
		account: normalizeMailAccountResponse(objectValue.account),
		mailboxes: normalizeMailboxesResponse({ mailboxes: objectValue.mailboxes }),
		messages: normalizeMailMessagesResponse({ messages: objectValue.messages }).messages,
		nextCursor: stringField(objectValue, 'nextCursor') ?? '',
		hasCachedMailboxes: booleanField(objectValue, 'hasCachedMailboxes') ?? false,
		hasCachedMessages: booleanField(objectValue, 'hasCachedMessages') ?? false
	};
}

export function normalizeMailMessageDetailResponse(value: unknown): Partial<MailMessage> {
	return normalizeMailMessageResponse(value) ?? {};
}

function normalizeMailboxResponse(value: unknown): Mailbox | undefined {
	const objectValue = objectRecord(value);
	if (!objectValue) return undefined;
	const name = stringField(objectValue, 'name');
	if (!name) return undefined;
	return {
		name,
		displayName: stringField(objectValue, 'displayName') ?? name,
		unseen: numberField(objectValue, 'unseen') ?? 0,
		total: numberField(objectValue, 'total') ?? 0
	};
}

function normalizeMailMessageResponse(value: unknown): MailMessage | undefined {
	const objectValue = objectRecord(value);
	if (!objectValue) return undefined;
	const uid = numberField(objectValue, 'uid');
	const mailbox = stringField(objectValue, 'mailbox');
	if (uid === undefined || !mailbox) return undefined;
	const message: MailMessage = {
		uid,
		mailbox,
		subject: stringField(objectValue, 'subject') ?? '',
		from: stringField(objectValue, 'from') ?? '',
		date: stringField(objectValue, 'date') ?? '',
		preview: stringField(objectValue, 'preview') ?? '',
		isRead: booleanField(objectValue, 'isRead') ?? false
	};
	const to = stringField(objectValue, 'to');
	if (to !== undefined) message.to = to;
	const cc = stringField(objectValue, 'cc');
	if (cc !== undefined) message.cc = cc;
	const body = stringField(objectValue, 'body');
	if (body !== undefined) message.body = body;
	const bodyHTML = stringField(objectValue, 'bodyHTML');
	if (bodyHTML !== undefined) message.bodyHTML = bodyHTML;
	return message;
}

function emptyMailBootstrap(): MailBootstrap {
	return {
		account: {},
		mailboxes: [],
		messages: [],
		nextCursor: '',
		hasCachedMailboxes: false,
		hasCachedMessages: false
	};
}

function objectRecord(value: unknown): Record<string, unknown> | undefined {
	if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
	return Object.fromEntries(Object.entries(value));
}

function stringField(objectValue: Record<string, unknown>, key: string) {
	const value = objectValue[key];
	return typeof value === 'string' ? value : undefined;
}

function numberField(objectValue: Record<string, unknown>, key: string) {
	const value = objectValue[key];
	return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function booleanField(objectValue: Record<string, unknown>, key: string) {
	const value = objectValue[key];
	return typeof value === 'boolean' ? value : undefined;
}

function isDefined<Value>(value: Value | undefined): value is Value {
	return value !== undefined;
}
