import { describe, expect, test } from 'bun:test';
import { x25519 } from '@noble/curves/ed25519.js';
import { readFileSync } from 'node:fs';
import type { SealedSecret } from '../../src/lib/company/box';
import { base64URLOf, bytesOfBase64URL } from '../../src/lib/company/seal-to-box';
import {
	asShown,
	asWritten,
	imapConnectionOf,
	mailPasswordPurpose,
	passwordSealer,
	passwordsLostToAServerChange,
	sealedPasswords,
	smtpConnectionOf,
	type MailAccount,
	type MailPasswordField
} from '../../src/lib/server/mail-account';
import { openedBy } from './company/open-from-box';

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

const boxSecretKey = x25519.utils.randomSecretKey();
const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
const owner = { companyID: 'company-a', memberID: 'member-a' };
const seal = passwordSealer(boxEncryptionKey, owner);

const imapConnection = { host: 'imap.example.com', port: 993, security: 'tls', username: 'first' };
const smtpConnection = { host: 'smtp.example.com', port: 587, security: 'starttls', username: 'first' };

function sealed(field: SealedSecret | null): SealedSecret {
	if (!field) throw new Error('the password was not sealed');
	return field;
}

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

describe('sealing the passwords to the company box', () => {
	test('what is kept holds no password anyone but the box can read', async () => {
		const kept = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);
		const stored = JSON.stringify(kept);

		expect(stored.includes('imap-secret')).toBe(false);
		expect(stored.includes('smtp-secret')).toBe(false);
		expect(JSON.parse(stored).IMAPPassword).toBe('');
		expect(JSON.parse(stored).SMTPPassword).toBe('');
		expect(await openedBy(boxSecretKey, sealed(kept.SealedIMAPPassword), mailPasswordPurpose(owner, 'IMAPPassword', imapConnection))).toBe(
			'imap-secret'
		);
		expect(await openedBy(boxSecretKey, sealed(kept.SealedSMTPPassword), mailPasswordPurpose(owner, 'SMTPPassword', smtpConnection))).toBe(
			'smtp-secret'
		);
	});

	test('a blank password keeps the sealed one exactly as it was', async () => {
		const held = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		const again = await sealedPasswords(
			asWritten({ ...written, displayName: '박예시', imapPassword: '', smtpPassword: '' }, 'first@example.com', held),
			seal
		);

		expect(again.SealedIMAPPassword).toEqual(held.SealedIMAPPassword);
		expect(again.SealedSMTPPassword).toEqual(held.SealedSMTPPassword);
		expect(again.DisplayName).toBe('박예시');
		expect(asShown(again)?.isConfigured).toBe(true);
		expect(passwordsLostToAServerChange(again, held)).toEqual([]);
	});

	test('a changed server, port, security or login with a blank password drops that password and says which', async () => {
		const held = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		for (const change of [
			{ imapHost: 'imap.changed.example.com' },
			{ imapPort: 143 },
			{ imapSecurity: 'none' },
			{ imapUsername: 'second' }
		]) {
			const moved = asWritten({ ...written, ...change, imapPassword: '', smtpPassword: '' }, 'first@example.com', held);

			expect(moved.SealedIMAPPassword).toBeNull();
			expect(moved.SealedSMTPPassword).toEqual(held.SealedSMTPPassword);
			expect(passwordsLostToAServerChange(moved, held)).toEqual(['IMAP']);
		}
	});

	test('a changed server with its password entered again keeps nothing to refuse', async () => {
		const held = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		const moved = asWritten(
			{ ...written, smtpHost: 'smtp.changed.example.com', imapPassword: '', smtpPassword: 'new-smtp' },
			'first@example.com',
			held
		);

		expect(passwordsLostToAServerChange(moved, held)).toEqual([]);
	});

	test('a password sealed for one server does not open for another', async () => {
		const kept = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		for (const elsewhere of [
			{ ...imapConnection, host: 'imap.attacker.test' },
			{ ...imapConnection, port: 143 },
			{ ...imapConnection, security: 'none' },
			{ ...imapConnection, username: 'second' },
			{ ...imapConnection, host: 'imap.example.com|first', username: '' }
		]) {
			await expect(
				openedBy(boxSecretKey, sealed(kept.SealedIMAPPassword), mailPasswordPurpose(owner, 'IMAPPassword', elsewhere))
			).rejects.toThrow();
		}
		expect(imapConnectionOf(kept)).toEqual(imapConnection);
		expect(smtpConnectionOf(kept)).toEqual(smtpConnection);
	});

	test('a new password is sealed afresh and the other is left alone', async () => {
		const held = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		const changed = await sealedPasswords(
			asWritten({ ...written, imapPassword: 'changed', smtpPassword: '' }, 'first@example.com', held),
			seal
		);

		expect(changed.SealedIMAPPassword).not.toEqual(held.SealedIMAPPassword);
		expect(changed.SealedSMTPPassword).toEqual(held.SealedSMTPPassword);
		expect(await openedBy(boxSecretKey, sealed(changed.SealedIMAPPassword), mailPasswordPurpose(owner, 'IMAPPassword', imapConnection))).toBe(
			'changed'
		);
	});

	test('a password kept readable before the company had a box is sealed the next time it is kept', async () => {
		const readable = asWritten(written, 'first@example.com', null);

		const kept = await sealedPasswords(readable, seal);

		expect(kept.IMAPPassword).toBe('');
		expect(kept.SealedIMAPPassword?.recipient).toBe(boxEncryptionKey);
	});

	test('a password sealed for one member does not open for another, nor as the other password', async () => {
		const kept = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);
		const imapPassword = sealed(kept.SealedIMAPPassword);

		const elsewhere: [typeof owner, MailPasswordField][] = [
			[{ companyID: 'company-a', memberID: 'member-b' }, 'IMAPPassword'],
			[{ companyID: 'company-b', memberID: 'member-a' }, 'IMAPPassword'],
			[owner, 'SMTPPassword']
		];
		for (const [someoneElse, field] of elsewhere) {
			await expect(openedBy(boxSecretKey, imapPassword, mailPasswordPurpose(someoneElse, field, imapConnection))).rejects.toThrow();
		}
	});

	test('a password sealed to one box does not open on another', async () => {
		const kept = await sealedPasswords(asWritten(written, 'first@example.com', null), seal);

		await expect(
			openedBy(x25519.utils.randomSecretKey(), sealed(kept.SealedIMAPPassword), mailPasswordPurpose(owner, 'IMAPPassword', imapConnection))
		).rejects.toThrow();
	});

	test('the password the box side opens in its tests opens here for the same member, field and server', async () => {
		const fixture = JSON.parse(
			readFileSync(new URL('../../../internal/box/testdata/sealed-mail-password.json', import.meta.url), 'utf8')
		);
		const purpose = mailPasswordPurpose(
			{ companyID: fixture.companyID, memberID: fixture.memberID },
			fixture.field,
			fixture.connection
		);

		expect(await openedBy(bytesOfBase64URL(fixture.boxSecretKey), fixture.sealed, purpose)).toBe(fixture.password);
	});
});
