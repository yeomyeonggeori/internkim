import type { ComposeDraft, ComposePayload, MailAccount, MailAccountDraft, MailAccountWritePayload } from './mail-types';

export const emptyMailAccount: MailAccount = {
	email: '',
	fromAddress: '',
	displayName: '',
	imapHost: '',
	imapPort: 993,
	imapSecurity: 'tls',
	imapUsername: '',
	smtpHost: '',
	smtpPort: 587,
	smtpSecurity: 'starttls',
	smtpUsername: '',
	defaultMailbox: 'INBOX',
	sentMailbox: 'Sent',
	isConfigured: false,
	hasIMAPPassword: false,
	hasSMTPPassword: false
};

export const emptyComposeDraft: ComposeDraft = {
	to: '',
	cc: '',
	bcc: '',
	subject: '',
	body: ''
};

export function createMailAccountDraft(account: MailAccount): MailAccountDraft {
	return { ...emptyMailAccount, ...account, imapPassword: '', smtpPassword: '' };
}

type MailServer = { host: string; port: number; security: string; username: string };

function imapServerOf(account: MailAccount): MailServer {
	return { host: account.imapHost.trim(), port: Number(account.imapPort), security: account.imapSecurity, username: account.imapUsername.trim() };
}

function smtpServerOf(account: MailAccount): MailServer {
	return { host: account.smtpHost.trim(), port: Number(account.smtpPort), security: account.smtpSecurity, username: account.smtpUsername.trim() };
}

function isSameServer(first: MailServer, second: MailServer): boolean {
	return (
		first.host === second.host &&
		first.port === second.port &&
		first.security === second.security &&
		first.username === second.username
	);
}

export function keepsSavedIMAPPassword(account: MailAccount, accountDraft: MailAccountDraft): boolean {
	return account.hasIMAPPassword && isSameServer(imapServerOf(account), imapServerOf(accountDraft));
}

export function keepsSavedSMTPPassword(account: MailAccount, accountDraft: MailAccountDraft): boolean {
	return account.hasSMTPPassword && isSameServer(smtpServerOf(account), smtpServerOf(accountDraft));
}

export function isPasswordNeededAgain(account: MailAccount, accountDraft: MailAccountDraft): boolean {
	const isIMAPLost = account.hasIMAPPassword && !keepsSavedIMAPPassword(account, accountDraft) && !accountDraft.imapPassword.trim();
	const isSMTPLost = account.hasSMTPPassword && !keepsSavedSMTPPassword(account, accountDraft) && !accountDraft.smtpPassword.trim();
	return isIMAPLost || isSMTPLost;
}

export function mailAccountDraftPayload(accountDraft: MailAccountDraft): MailAccountWritePayload {
	return {
		email: accountDraft.email,
		fromAddress: accountDraft.email,
		displayName: accountDraft.displayName,
		imapHost: accountDraft.imapHost,
		imapPort: Number(accountDraft.imapPort),
		imapSecurity: accountDraft.imapSecurity,
		imapUsername: accountDraft.imapUsername,
		imapPassword: accountDraft.imapPassword,
		smtpHost: accountDraft.smtpHost,
		smtpPort: Number(accountDraft.smtpPort),
		smtpSecurity: accountDraft.smtpSecurity,
		smtpUsername: accountDraft.smtpUsername,
		smtpPassword: accountDraft.smtpPassword,
		defaultMailbox: accountDraft.defaultMailbox,
		sentMailbox: accountDraft.sentMailbox
	};
}

export function composeDraftPayload(composeDraft: ComposeDraft): ComposePayload {
	return {
		to: splitMailAddressList(composeDraft.to),
		cc: splitMailAddressList(composeDraft.cc),
		bcc: splitMailAddressList(composeDraft.bcc),
		subject: composeDraft.subject,
		body: composeDraft.body
	};
}

export function splitMailAddressList(value: string) {
	return value
		.split(',')
		.map((address) => address.trim())
		.filter(Boolean);
}
