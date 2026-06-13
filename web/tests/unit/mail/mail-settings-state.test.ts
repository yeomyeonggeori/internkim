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
		expect(mailSettingsEmailStateFromEmail('mohyeong@example.net')).toEqual({
			emailLocalPart: 'mohyeong',
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

		expect(mailAddressSettingsUpdate(accountDraft, 'mohyeong', 'gmail', '', false)).toMatchObject({
			email: 'mohyeong@example.net',
			fromAddress: 'mohyeong@example.net',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com',
			imapUsername: 'mohyeong@example.net',
			smtpUsername: 'mohyeong@example.net',
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
