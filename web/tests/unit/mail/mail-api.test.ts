import { describe, expect, test } from 'bun:test';

import {
	normalizeMailAccountResponse,
	normalizeMailBootstrapResponse,
	normalizeMailboxesResponse,
	normalizeMailMessageDetailResponse,
	normalizeMailMessagesResponse
} from '../../../src/routes/mail/mail-api-normalizers';

describe('mail api response normalizers', () => {
	test('keeps only typed account fields', () => {
		expect(
			normalizeMailAccountResponse({
				email: 'staff@example.com',
				imapPort: 993,
				smtpPort: '587',
				isConfigured: true,
				hasIMAPPassword: 1,
				hasSMTPPassword: false
			})
		).toEqual({
			email: 'staff@example.com',
			imapPort: 993,
			isConfigured: true,
			hasSMTPPassword: false
		});
	});

	test('drops malformed mailboxes', () => {
		expect(
			normalizeMailboxesResponse({
				mailboxes: [
					{ name: 'INBOX', displayName: 'Inbox', unseen: 2, total: 10 },
					{ displayName: 'Missing name', unseen: 1 },
					{ name: 'Archive', unseen: 'bad', total: 3 }
				]
			})
		).toEqual([
			{ name: 'INBOX', displayName: 'Inbox', unseen: 2, total: 10 },
			{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 3 }
		]);
	});

	test('normalizes list messages and cursor', () => {
		expect(
			normalizeMailMessagesResponse({
				messages: [
					{ uid: 1, mailbox: 'INBOX', subject: 'Subject', from: 'sender@example.com', date: 'today', preview: 'hello', isRead: true },
					{ uid: '2', mailbox: 'INBOX' }
				],
				nextCursor: 'cursor'
			})
		).toEqual({
			messages: [{ uid: 1, mailbox: 'INBOX', subject: 'Subject', from: 'sender@example.com', date: 'today', preview: 'hello', isRead: true }],
			nextCursor: 'cursor'
		});
	});

	test('normalizes message detail body fields', () => {
		expect(
			normalizeMailMessageDetailResponse({
				uid: 1,
				mailbox: 'INBOX',
				body: 'plain',
				bodyHTML: '<p>html</p>',
				isRead: false
			})
		).toEqual({
			uid: 1,
			mailbox: 'INBOX',
			subject: '',
			from: '',
			date: '',
			preview: '',
			body: 'plain',
			bodyHTML: '<p>html</p>',
			isRead: false
		});
	});

	test('normalizes bootstrap cached state', () => {
		expect(
			normalizeMailBootstrapResponse({
				account: { email: 'staff@example.com', isConfigured: true },
				mailboxes: [{ name: 'INBOX', displayName: 'Inbox', unseen: 1, total: 3 }],
				messages: [{ uid: 3, mailbox: 'INBOX', subject: 'Cached' }],
				nextCursor: 'older',
				hasCachedMailboxes: true,
				hasCachedMessages: true
			})
		).toEqual({
			account: { email: 'staff@example.com', isConfigured: true },
			mailboxes: [{ name: 'INBOX', displayName: 'Inbox', unseen: 1, total: 3 }],
			messages: [{ uid: 3, mailbox: 'INBOX', subject: 'Cached', from: '', date: '', preview: '', isRead: false }],
			nextCursor: 'older',
			hasCachedMailboxes: true,
			hasCachedMessages: true
		});
	});
});
