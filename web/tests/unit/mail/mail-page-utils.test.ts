import { describe, expect, test } from 'bun:test';

import { emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import {
	createMailReplyDraft,
	defaultMailboxes,
	displayedMailboxes,
	mailApiErrorMessages,
	mailboxNameByHint,
	mailboxUnreadCount,
	mailboxUnreadCountText,
	mailMessageBody,
	mailMessageBodyHTML,
	selectedMailboxCountText,
	visibleMailMessages
} from '../../../src/routes/mail/mail-page-utils';
import type { MailMessage } from '../../../src/routes/mail/mail-types';
import { mailText } from '../../../src/routes/mail/text';

const baseMessage: MailMessage = {
	uid: 1,
	mailbox: 'INBOX',
	subject: 'Subject',
	from: 'sender@example.com',
	date: '',
	preview: 'preview',
	isRead: false
};

describe('mail page utils', () => {
	test('creates localized fallback mailboxes', () => {
		expect(defaultMailboxes(mailText.ko).map((mailbox) => mailbox.displayName)).toEqual(['받은편지함', '보낸메일', '임시보관함', '보관함', '휴지통']);
	});

	test('uses remote mailboxes only for configured accounts', () => {
		const fallbackMailboxes = defaultMailboxes(mailText.ko);
		const remoteMailboxes = [{ name: 'Remote', displayName: 'Remote', unseen: 0, total: 1 }];

		expect(displayedMailboxes(emptyMailAccount, remoteMailboxes, fallbackMailboxes)).toBe(fallbackMailboxes);
		expect(displayedMailboxes({ ...emptyMailAccount, isConfigured: true }, remoteMailboxes, fallbackMailboxes)).toBe(remoteMailboxes);
	});

	test('filters unread messages', () => {
		const messages = [baseMessage, { ...baseMessage, uid: 2, isRead: true }];

		expect(visibleMailMessages(messages, true)).toEqual([baseMessage]);
	});

	test('formats mailbox counts from server mailbox status', () => {
		const mailboxes = [{ name: 'INBOX', displayName: 'INBOX', unseen: 3, total: 10 }];

		expect(mailboxUnreadCount(mailboxes[0])).toBe(3);
		expect(selectedMailboxCountText(mailboxes, 'INBOX', mailText.ko)).toBe('전체 10개 · 읽지 않음 3개');
		expect(mailboxUnreadCountText(mailboxes[0], mailText.ko)).toBe('읽지 않음 3개');
		expect(selectedMailboxCountText(mailboxes, 'Sent', mailText.ko)).toBe('');
	});

	test('keeps server mailbox counts independent', () => {
		const mailbox = { name: 'INBOX', displayName: 'INBOX', unseen: 12, total: 10 };

		expect(selectedMailboxCountText([mailbox], 'INBOX', mailText.en)).toBe('Total 10 · Unread 12');
	});

	test('does not show negative unread counts', () => {
		const mailbox = { name: 'INBOX', displayName: 'INBOX', unseen: -1, total: 10 };

		expect(mailboxUnreadCount(mailbox)).toBe(0);
		expect(mailboxUnreadCountText(mailbox, mailText.ko)).toBe('읽지 않음 0개');
		expect(selectedMailboxCountText([mailbox], 'INBOX', mailText.ko)).toBe('전체 10개 · 읽지 않음 0개');
	});

	test('builds message body and reply draft values', () => {
		const message = { ...baseMessage, body: 'body', bodyHTML: ' <p>html</p> ' };

		expect(mailMessageBody(message)).toBe('body');
		expect(mailMessageBodyHTML(message)).toBe('<p>html</p>');
		expect(createMailReplyDraft(message)).toEqual({ to: 'sender@example.com', cc: '', bcc: '', subject: 'Re: Subject', body: '' });
		expect(createMailReplyDraft({ ...message, subject: 're: Subject' }).subject).toBe('re: Subject');
	});

	test('finds mailbox names by hint and builds api error messages', () => {
		const mailboxes = defaultMailboxes(mailText.ko);

		expect(mailboxNameByHint(mailboxes, 'trash')).toBe('Trash');
		expect(mailApiErrorMessages('fallback', 'unavailable')).toEqual({ fallback: 'fallback', serviceUnavailable: 'unavailable' });
	});
});
