import { describe, expect, test } from 'bun:test';

import { createMailAccountDraft, emptyMailAccount } from '../../../src/routes/mail/mail-account-draft';
import {
	mailAddressSettingsUpdate,
	mailProviderSettingsUpdate,
	mailSettingsEmailStateFromEmail,
	selectedMailSettingsDomain
} from '../../../src/routes/mail/mail-settings-state';

describe('mail settings state', () => {
	test('creates sheet email state from saved addresses', () => {
		expect(mailSettingsEmailStateFromEmail('chanhee@gmail.com')).toEqual({
			emailLocalPart: 'chanhee',
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

		expect(mailAddressSettingsUpdate(accountDraft, 'chanhee', 'gmail', '', false)).toMatchObject({
			email: 'chanhee@gmail.com',
			fromAddress: 'chanhee@gmail.com',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com',
			imapUsername: 'chanhee@gmail.com',
			smtpUsername: 'chanhee@gmail.com',
			sentMailbox: ''
		});
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
		expect(selectedMailSettingsDomain('custom', 'example.com')).toBe('example.com');
	});
});
