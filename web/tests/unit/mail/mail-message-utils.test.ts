import { describe, expect, test } from 'bun:test';

import { mailHTMLDocument, mailMessageKey, mergeMailMessages } from '../../../src/routes/mail/mail-message-utils';
import type { MailMessage } from '../../../src/routes/mail/mail-types';

const baseMessage: MailMessage = {
	uid: 1,
	mailbox: 'INBOX',
	subject: 'Subject',
	from: 'sender@example.com',
	date: '',
	preview: '',
	isRead: false
};

describe('mail message utils', () => {
	test('keys messages by mailbox and uid', () => {
		expect(mailMessageKey({ ...baseMessage, mailbox: 'Archive', uid: 42 })).toBe('Archive:42');
	});

	test('merges incoming messages without duplicating existing keys', () => {
		const existingMessages = [{ ...baseMessage, uid: 1 }];
		const incomingMessages = [
			{ ...baseMessage, uid: 1, subject: 'Duplicate' },
			{ ...baseMessage, uid: 2, subject: 'New' }
		];

		expect(mergeMailMessages(existingMessages, incomingMessages)).toEqual([
			{ ...baseMessage, uid: 1 },
			{ ...baseMessage, uid: 2, subject: 'New' }
		]);
	});

	test('wraps message html in a sandbox-friendly document', () => {
		const document = mailHTMLDocument('<p>Hello</p>');

		expect(document.includes('<meta http-equiv="Content-Security-Policy"')).toBe(true);
		expect(document.includes('img-src data: cid:')).toBe(true);
		expect(document.includes('img-src https:')).toBe(false);
		expect(document.includes('img-src http:')).toBe(false);
		expect(document.includes("script-src 'none'")).toBe(true);
		expect(document.includes('<body><p>Hello</p></body>')).toBe(true);
	});
});
