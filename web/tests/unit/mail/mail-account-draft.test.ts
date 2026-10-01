import { describe, expect, test } from 'bun:test';

import {
	composeDraftPayload,
	createMailAccountDraft,
	emptyMailAccount,
	isPasswordNeededAgain,
	keepsSavedIMAPPassword,
	keepsSavedSMTPPassword,
	mailAccountDraftPayload,
	splitMailAddressList
} from '../../../src/routes/mail/mail-account-draft';

describe('mail account draft', () => {
	test('creates an editable draft without saved passwords', () => {
		const draft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'member@example.com',
			hasIMAPPassword: true,
			hasSMTPPassword: true
		});

		expect(draft.email).toBe('member@example.com');
		expect(draft.imapPassword).toBe('');
		expect(draft.smtpPassword).toBe('');
		expect(draft.hasIMAPPassword).toBe(true);
		expect(draft.hasSMTPPassword).toBe(true);
	});

	test('builds account save payload from draft values', () => {
		const payload = mailAccountDraftPayload({
			...createMailAccountDraft(emptyMailAccount),
			email: 'member@example.com',
			displayName: 'Member',
			imapPort: 1993,
			imapPassword: 'imap-secret',
			smtpPort: 2587,
			smtpPassword: 'smtp-secret'
		});

		expect(payload).toEqual({
			email: 'member@example.com',
			fromAddress: 'member@example.com',
			displayName: 'Member',
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

	test('preserves password whitespace in account payload', () => {
		const payload = mailAccountDraftPayload({
			...createMailAccountDraft(emptyMailAccount),
			imapPassword: 'abcd efgh ijkl mnop',
			smtpPassword: 'qrst uvwx yzab cdef'
		});

		expect(payload.imapPassword).toBe('abcd efgh ijkl mnop');
		expect(payload.smtpPassword).toBe('qrst uvwx yzab cdef');
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

describe('a saved password and the server it belongs to', () => {
	const saved = {
		...emptyMailAccount,
		imapHost: 'imap.example.com',
		imapUsername: 'first',
		smtpHost: 'smtp.example.com',
		smtpUsername: 'first',
		isConfigured: true,
		hasIMAPPassword: true,
		hasSMTPPassword: true
	};

	test('the saved password stays when the server does', () => {
		const draft = { ...createMailAccountDraft(saved), displayName: '박예시' };

		expect(keepsSavedIMAPPassword(saved, draft)).toBe(true);
		expect(isPasswordNeededAgain(saved, draft)).toBe(false);
	});

	test('a changed server, port, security or login asks for the password again', () => {
		for (const change of [
			{ imapHost: 'imap.changed.example.com' },
			{ imapPort: 143 },
			{ imapSecurity: 'none' },
			{ smtpUsername: 'second' }
		]) {
			const draft = { ...createMailAccountDraft(saved), ...change };

			expect(isPasswordNeededAgain(saved, draft)).toBe(true);
			expect(isPasswordNeededAgain(saved, { ...draft, imapPassword: 'new', smtpPassword: 'new' })).toBe(false);
		}
	});

	test('the SMTP field shows the saved password only while the SMTP server is unchanged', () => {
		const draft = { ...createMailAccountDraft(saved), smtpHost: 'smtp.changed.example.com' };

		expect(keepsSavedSMTPPassword(saved, draft)).toBe(false);
		expect(keepsSavedIMAPPassword(saved, draft)).toBe(true);
	});
});
