import { describe, expect, test } from 'bun:test';
import { asShown, asWritten, type MailAccount } from '../../src/lib/server/mail-account';

const written = {
	email: 'first@example.com',
	displayName: '이샘플',
	imapHost: 'imap.example.com',
	imapPort: 993,
	imapSecurity: 'tls',
	imapUsername: 'first',
	imapPassword: 'imap-secret',
	smtpHost: 'smtp.example.com',
	smtpPort: 587,
	smtpSecurity: 'starttls',
	smtpUsername: 'first',
	smtpPassword: 'smtp-secret',
	defaultMailbox: 'INBOX',
	sentMailbox: '보낸편지함'
};

describe('asWritten', () => {
	test('what the mail tab already sends becomes what maild already takes', () => {
		const account = asWritten(written, 'first@example.com', null);

		expect(account.IMAPHost).toBe('imap.example.com');
		expect(account.SMTPPort).toBe(587);
		expect(account.SentMailbox).toBe('보낸편지함');
		expect(account.ActorEmail).toBe('first@example.com');
	});

	test('a blank password keeps the one already stored, so saving a host does not lose it', () => {
		const held = asWritten(written, 'first@example.com', null);

		const again = asWritten({ ...written, imapPassword: '', smtpPassword: '' }, 'first@example.com', held);

		expect(again.IMAPPassword).toBe('imap-secret');
		expect(again.SMTPPassword).toBe('smtp-secret');
	});

	test('a new password replaces the old one', () => {
		const held = asWritten(written, 'first@example.com', null);
		const changed = asWritten({ ...written, imapPassword: 'changed' }, 'first@example.com', held);

		expect(changed.IMAPPassword).toBe('changed');
	});

	test('an account arrives with the ports and mailboxes a person would otherwise have to know', () => {
		const account = asWritten(
			{ imapHost: 'imap.example.com', smtpHost: 'smtp.example.com' },
			'first@example.com',
			null
		);

		expect(account.IMAPPort).toBe(993);
		expect(account.SMTPPort).toBe(587);
		expect(account.IMAPSecurity).toBe('tls');
		expect(account.SMTPSecurity).toBe('starttls');
		expect(account.DefaultMailbox).toBe('INBOX');
		expect(account.SentMailbox).toBe('Sent');
	});

	test('a port of the wrong kind is not a port', () => {
		const account = asWritten({ ...written, imapPort: '993', smtpPort: 70000 }, 'first@example.com', null);

		expect(account.IMAPPort).toBe(993);
		expect(account.SMTPPort).toBe(587);
	});
});

describe('asShown', () => {
	test('no password crosses back to the browser', () => {
		const shown = asShown(asWritten(written, 'first@example.com', null));

		expect(JSON.stringify(shown).includes('secret')).toBe(false);
		expect(shown?.hasIMAPPassword).toBe(true);
		expect(shown?.hasSMTPPassword).toBe(true);
	});

	test('an account missing a password is shown as not yet configured', () => {
		const account: MailAccount = { ...asWritten(written, 'first@example.com', null), SMTPPassword: '' };

		expect(asShown(account)?.isConfigured).toBe(false);
		expect(asShown(account)?.hasSMTPPassword).toBe(false);
	});

	test('a member who has connected nothing is shown nothing', () => {
		expect(asShown(null)).toBeNull();
	});
});
