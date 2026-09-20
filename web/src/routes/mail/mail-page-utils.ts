import type { ComposeDraft, MailAccount, Mailbox, MailMessage } from './mail-types';
import type { mailText } from './text';
import type { PageText } from '$lib/i18n/page-text.svelte';

type MailPageText = PageText<typeof mailText>;

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

export function selectedMailboxLabel(mailboxes: Mailbox[], selectedMailbox: string) {
	const mailbox = mailboxes.find((candidateMailbox) => candidateMailbox.name === selectedMailbox);
	return mailbox?.displayName || selectedMailbox;
}

export function mailboxUnreadCount(mailbox: Mailbox) {
	return Math.max(mailbox.unseen, 0);
}

export function mailboxUnreadCountText(mailbox: Mailbox, text: MailPageText) {
	return text.mailboxUnreadCount.replace('{count}', String(mailboxUnreadCount(mailbox)));
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

export function createMailForwardDraft(message: MailMessage, quotedHeader: string): ComposeDraft {
	return {
		to: '',
		cc: '',
		bcc: '',
		subject: message.subject.toLowerCase().startsWith('fwd:') ? message.subject : `Fwd: ${message.subject}`,
		body: `\n\n${quotedHeader}\n${mailMessageBody(message)}`
	};
}

export const MAIL_MOVE_TARGET_HINTS = {
	archive: ['archive', '보관'],
	junk: ['junk', 'spam', '스팸'],
	trash: ['trash', 'deleted', '휴지통']
} as const satisfies Record<string, readonly string[]>;

export type MailMoveTarget = keyof typeof MAIL_MOVE_TARGET_HINTS;

export function mailboxNameByHint(mailboxes: Mailbox[], hints: readonly string[]) {
	const normalizedHints = hints.map((hint) => hint.toLowerCase());
	return mailboxes.find((mailbox) => normalizedHints.some((hint) => mailbox.name.toLowerCase().includes(hint)))?.name;
}

export function availableMailMoveTargets(mailboxes: Mailbox[]) {
	const targets = Object.keys(MAIL_MOVE_TARGET_HINTS) as MailMoveTarget[];
	return targets.filter((target) => mailboxNameByHint(mailboxes, MAIL_MOVE_TARGET_HINTS[target]));
}

export function mailApiErrorMessages(fallback: string, serviceUnavailable: string) {
	return { fallback, serviceUnavailable };
}

