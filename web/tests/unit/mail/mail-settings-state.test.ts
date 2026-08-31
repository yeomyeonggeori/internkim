import { describe, expect, test } from 'bun:test';

import { createMailAccountDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import {
	currentMailPresetFromDraft,
	mailAppPasswordInputValue,
	mailAddressSettingsUpdate,
	mailProviderSettingsUpdate,
	mailSettingsEmailStateFromEmail,
	selectedMailSettingsDomain
} from '../../../src/routes/mail/mail-settings-state';

describe('mail settings state', () => {
	test('creates sheet email state from saved addresses', () => {
		expect(mailSettingsEmailStateFromEmail('sample@gmail.com')).toEqual({
			emailLocalPart: 'sample',
			emailProviderID: 'gmail',
			customEmailDomain: '',
			isAdvancedSettingsOpen: false
		});
		expect(mailSettingsEmailStateFromEmail('member@example.com')).toEqual({
			emailLocalPart: 'member',
			emailProviderID: 'custom',
			customEmailDomain: 'example.com',
			isAdvancedSettingsOpen: true
		});
	});

	test('fills Gmail server settings from the selected domain', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'sample', 'gmail', '', false)).toMatchObject({
			email: 'sample@gmail.com',
			fromAddress: 'sample@gmail.com',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com',
			imapUsername: 'sample@gmail.com',
			smtpUsername: 'sample@gmail.com',
			sentMailbox: ''
		});
	});

	test('fills Daum server settings with the login ID only', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'sample', 'daum', '', false)).toMatchObject({
			email: 'sample@daum.net',
			fromAddress: 'sample@daum.net',
			imapHost: 'imap.daum.net',
			imapPort: 993,
			imapSecurity: 'tls',
			smtpHost: 'smtp.daum.net',
			smtpPort: 465,
			smtpSecurity: 'tls',
			imapUsername: 'sample',
			smtpUsername: 'sample'
		});
	});

	test('fills Hanmail server settings with the login ID only', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'sample', 'hanmail', '', false)).toMatchObject({
			email: 'sample@hanmail.net',
			fromAddress: 'sample@hanmail.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net',
			imapUsername: 'sample',
			smtpUsername: 'sample'
		});
	});

	test('detects Daum and Hanmail presets by the email domain when server settings match', () => {
		const daumDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'sample@daum.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net'
		});
		const hanmailDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'sample@hanmail.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net'
		});

		expect(currentMailPresetFromDraft(daumDraft)?.id).toBe('daum');
		expect(currentMailPresetFromDraft(hanmailDraft)?.id).toBe('hanmail');
	});

	test('preserves manual server settings while editing the address', () => {
		const accountDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'member@example.com',
			imapHost: 'imap.example.com',
			smtpHost: 'smtp.example.com'
		});

		expect(mailAddressSettingsUpdate(accountDraft, 'member', 'gmail', '', false)).toEqual({
			email: 'member@gmail.com',
			fromAddress: 'member@gmail.com',
			imapUsername: 'member@gmail.com',
			smtpUsername: 'member@gmail.com'
		});
	});

	test('clears known provider hosts when switching to custom', () => {
		const accountDraft = createMailAccountDraft({
			...emptyMailAccount,
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com',
			sentMailbox: ''
		});

		expect(mailProviderSettingsUpdate(accountDraft, 'custom')).toEqual({
			imapHost: '',
			smtpHost: '',
			sentMailbox: 'Sent'
		});
	});

	test('resolves the domain shown in the split email control', () => {
		expect(selectedMailSettingsDomain('gmail', '')).toBe('gmail.com');
		expect(selectedMailSettingsDomain('daum', '')).toBe('daum.net');
		expect(selectedMailSettingsDomain('hanmail', '')).toBe('hanmail.net');
		expect(selectedMailSettingsDomain('custom', 'example.com')).toBe('example.com');
	});

	test('normalizes copied Gmail app password input spaces', () => {
		expect(mailAppPasswordInputValue('abcd efgh ijkl mnop', 'gmail')).toBe('abcdefghijklmnop');
	});

	test('preserves custom provider password input spaces', () => {
		expect(mailAppPasswordInputValue('custom password with spaces', 'custom')).toBe('custom password with spaces');
	});
});
