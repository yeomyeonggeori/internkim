import { describe, expect, test } from 'bun:test';

import {
	MAIL_MESSAGE_IFRAME_SANDBOX,
	mailHTMLDocument,
	mailMessageKey,
	mailMessageTimeLabel,
	mergeMailMessages
} from '../../../src/routes/mail/mail-message-utils';
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
	test('labels received time by today, yesterday, and older dates', () => {
		const now = new Date(2026, 6, 28, 15, 0, 0);

		expect(mailMessageTimeLabel(new Date(2026, 6, 28, 9, 5).toISOString(), '어제', now, 'en-US')).toBe('9:05 AM');
		expect(mailMessageTimeLabel(new Date(2026, 6, 27, 23, 59).toISOString(), '어제', now, 'en-US')).toBe('어제');
		expect(mailMessageTimeLabel(new Date(2026, 6, 26, 12, 0).toISOString(), '어제', now, 'en-US')).toBe('7/26');
		expect(mailMessageTimeLabel(new Date(2025, 11, 24, 12, 0).toISOString(), '어제', now, 'en-US')).toBe('12/24/2025');
		expect(mailMessageTimeLabel('', '어제', now, 'en-US')).toBe('');
		expect(mailMessageTimeLabel('not-a-date', '어제', now, 'en-US')).toBe('');
	});

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
		const document = mailHTMLDocument('<p>Hello</p><a href="https://example.com">Open</a>');

		expect(document.includes('<meta http-equiv="Content-Security-Policy"')).toBe(true);
		expect(document.includes('img-src https: http: data: cid:')).toBe(true);
		expect(document.includes("script-src 'none'")).toBe(true);
		expect(document.includes('<body><p>Hello</p><a rel="noopener noreferrer" target="_blank" href="https://example.com">Open</a></body>')).toBe(true);
	});

	test('forces message links to open outside the app shell', () => {
		const document = mailHTMLDocument('<a target="_self" rel="nofollow" href="https://example.com">Open</a>');

		expect(document.includes('<body><a target="_blank" rel="nofollow noopener noreferrer" href="https://example.com">Open</a></body>')).toBe(true);
	});

	test('normalizes unquoted rel attributes when forcing safe link targets', () => {
		const document = mailHTMLDocument('<a rel=opener href="https://example.com">Open</a>');

		expect(document.includes('<body><a target="_blank" rel="opener noopener noreferrer" href="https://example.com">Open</a></body>')).toBe(true);
	});

	test('allows only link popups from the message iframe sandbox', () => {
		expect(MAIL_MESSAGE_IFRAME_SANDBOX).toBe('allow-popups allow-popups-to-escape-sandbox');
	});
});
