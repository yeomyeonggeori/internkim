import { describe, expect, test } from 'bun:test';

import { emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import {
	createMailReplyDraft,
	defaultMailboxes,
	displayedMailboxes,
	mailApiErrorMessages,
	mailboxNameByHint,
	mailMessageBody,
	mailMessageBodyHTML,
	mailMessageCountText,
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

	test('filters unread messages and formats count', () => {
		const messages = [baseMessage, { ...baseMessage, uid: 2, isRead: true }];

		expect(visibleMailMessages(messages, true)).toEqual([baseMessage]);
		expect(mailMessageCountText(messages, '개 메시지')).toBe('2개 메시지');
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
