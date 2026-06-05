import { describe, expect, test } from 'bun:test';

import {
	composeDraftPayload,
	createMailAccountDraft,
	emptyMailAccount,
	mailAccountDraftPayload,
	splitMailAddressList
} from '../../../src/routes/mail/mail-account-draft';

describe('mail account draft', () => {
	test('creates an editable draft without saved passwords', () => {
		const draft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'staff@example.com',
			hasIMAPPassword: true,
			hasSMTPPassword: true
		});

		expect(draft.email).toBe('staff@example.com');
		expect(draft.imapPassword).toBe('');
		expect(draft.smtpPassword).toBe('');
		expect(draft.hasIMAPPassword).toBe(true);
		expect(draft.hasSMTPPassword).toBe(true);
	});

	test('builds account save payload from draft values', () => {
		const payload = mailAccountDraftPayload({
			...createMailAccountDraft(emptyMailAccount),
			email: 'staff@example.com',
			displayName: 'Staff',
			imapPort: 1993,
			imapPassword: 'imap-secret',
			smtpPort: 2587,
			smtpPassword: 'smtp-secret'
		});

		expect(payload).toEqual({
			email: 'staff@example.com',
			fromAddress: 'staff@example.com',
			displayName: 'Staff',
			imapHost: '',
			imapPort: 1993,
			imapSecurity: 'tls',
			imapUsername: '',
			imapPassword: 'imap-secret',
			smtpHost: '',
			smtpPort: 2587,
			smtpSecurity: 'starttls',
			smtpUsername: '',
			smtpPassword: 'smtp-secret',
			defaultMailbox: 'INBOX',
			sentMailbox: 'Sent'
		});
	});

	test('trims compose address lists', () => {
		expect(splitMailAddressList(' a@example.com, ,b@example.com ')).toEqual(['a@example.com', 'b@example.com']);
		expect(
			composeDraftPayload({
				to: 'a@example.com, b@example.com',
				cc: '',
				bcc: 'c@example.com',
				subject: 'Subject',
				body: 'Body'
			})
		).toEqual({
			to: ['a@example.com', 'b@example.com'],
			cc: [],
			bcc: ['c@example.com'],
			subject: 'Subject',
			body: 'Body'
		});
	});
});
