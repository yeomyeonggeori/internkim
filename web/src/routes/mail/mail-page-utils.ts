import type { ComposeDraft, MailAccount, Mailbox, MailMessage } from './mail-types';
import type { mailText } from './text';

type MailPageText = (typeof mailText)['ko'];

export function defaultMailboxes(text: MailPageText): Mailbox[] {
	return [
		{ name: 'INBOX', displayName: text.defaultMailboxes.inbox, unseen: 0, total: 0 },
		{ name: 'Sent', displayName: text.defaultMailboxes.sent, unseen: 0, total: 0 },
		{ name: 'Drafts', displayName: text.defaultMailboxes.drafts, unseen: 0, total: 0 },
		{ name: 'Archive', displayName: text.defaultMailboxes.archive, unseen: 0, total: 0 },
		{ name: 'Trash', displayName: text.defaultMailboxes.trash, unseen: 0, total: 0 }
	];
}

export function displayedMailboxes(account: MailAccount, mailboxes: Mailbox[], fallbackMailboxes: Mailbox[]) {
	if (account.isConfigured && mailboxes.length) return mailboxes;
	return fallbackMailboxes;
}

export function visibleMailMessages(messages: MailMessage[], isUnreadOnly: boolean) {
	return messages.filter((message) => !isUnreadOnly || !message.isRead);
}

export function mailMessageCountText(messages: MailMessage[], suffix: string) {
	return `${messages.length}${suffix}`;
}

export function mailMessageBody(message: MailMessage | null) {
	return message?.body || message?.preview || '';
}

export function mailMessageBodyHTML(message: MailMessage | null) {
	return message?.bodyHTML?.trim() || '';
}

export function createMailReplyDraft(message: MailMessage): ComposeDraft {
	return {
		to: message.from,
		cc: '',
		bcc: '',
		subject: message.subject.toLowerCase().startsWith('re:') ? message.subject : `Re: ${message.subject}`,
		body: ''
	};
}

export function mailboxNameByHint(mailboxes: Mailbox[], hint: string) {
	const normalizedHint = hint.toLowerCase();
	return mailboxes.find((mailbox) => mailbox.name.toLowerCase().includes(normalizedHint))?.name;
}

export function mailApiErrorMessages(fallback: string, serviceUnavailable: string) {
	return { fallback, serviceUnavailable };
}
