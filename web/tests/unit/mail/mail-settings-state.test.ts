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
		expect(mailSettingsEmailStateFromEmail('gyeonbon@gmail.com')).toEqual({
			emailLocalPart: 'gyeonbon',
			emailProviderID: 'gmail',
			customEmailDomain: '',
			isAdvancedSettingsOpen: false
		});
		expect(mailSettingsEmailStateFromEmail('staff@example.com')).toEqual({
			emailLocalPart: 'staff',
			emailProviderID: 'custom',
			customEmailDomain: 'example.com',
			isAdvancedSettingsOpen: true
		});
	});

	test('fills Gmail server settings from the selected domain', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'gyeonbon', 'gmail', '', false)).toMatchObject({
			email: 'gyeonbon@gmail.com',
			fromAddress: 'gyeonbon@gmail.com',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com',
			imapUsername: 'gyeonbon@gmail.com',
			smtpUsername: 'gyeonbon@gmail.com',
			sentMailbox: ''
		});
	});

	test('fills Daum server settings with the login ID only', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'chanee234', 'daum', '', false)).toMatchObject({
			email: 'chanee234@daum.net',
			fromAddress: 'chanee234@daum.net',
			imapHost: 'imap.daum.net',
			imapPort: 993,
			imapSecurity: 'tls',
			smtpHost: 'smtp.daum.net',
			smtpPort: 465,
			smtpSecurity: 'tls',
			imapUsername: 'chanee234',
			smtpUsername: 'chanee234'
		});
	});

	test('fills Hanmail server settings with the login ID only', () => {
		const accountDraft = createMailAccountDraft(emptyMailAccount);

		expect(mailAddressSettingsUpdate(accountDraft, 'chanee234', 'hanmail', '', false)).toMatchObject({
			email: 'chanee234@hanmail.net',
			fromAddress: 'chanee234@hanmail.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net',
			imapUsername: 'chanee234',
			smtpUsername: 'chanee234'
		});
	});

	test('detects Daum and Hanmail presets by the email domain when server settings match', () => {
		const daumDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'chanee234@daum.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net'
		});
		const hanmailDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'chanee234@hanmail.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net'
		});

		expect(currentMailPresetFromDraft(daumDraft)?.id).toBe('daum');
		expect(currentMailPresetFromDraft(hanmailDraft)?.id).toBe('hanmail');
	});

	test('preserves manual server settings while editing the address', () => {
		const accountDraft = createMailAccountDraft({
			...emptyMailAccount,
			email: 'staff@example.com',
			imapHost: 'imap.example.com',
			smtpHost: 'smtp.example.com'
		});

		expect(mailAddressSettingsUpdate(accountDraft, 'staff', 'gmail', '', false)).toEqual({
			email: 'staff@gmail.com',
			fromAddress: 'staff@gmail.com',
			imapUsername: 'staff@gmail.com',
			smtpUsername: 'staff@gmail.com'
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
