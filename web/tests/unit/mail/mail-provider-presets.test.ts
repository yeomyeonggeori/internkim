import { describe, expect, test } from 'bun:test';

import {
	composeMailAddress,
	mailAddressDraftFromEmail,
	mailProviderIDFromDomain,
	mailProviderPreset
} from '../../../src/routes/mail/mail-provider-presets';

describe('mail provider presets', () => {
	test('resolves known mail domains', () => {
		expect(mailProviderIDFromDomain('gmail.com')).toBe('gmail');
		expect(mailProviderIDFromDomain('NAVER.COM')).toBe('naver');
		expect(mailProviderIDFromDomain('example.com')).toBe('custom');
	});

	test('builds email address draft from full addresses', () => {
		expect(mailAddressDraftFromEmail('')).toEqual({
			localPart: '',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail('mohyeong')).toEqual({
			localPart: 'mohyeong',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail(' mohyeong@example.net ')).toEqual({
			localPart: 'mohyeong',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail('staff@example.com')).toEqual({
			localPart: 'staff',
			providerID: 'custom',
			customDomain: 'example.com'
		});
	});

	test('composes normalized mail addresses', () => {
		expect(composeMailAddress(' mohyeong ', ' Gmail.COM ')).toBe('mohyeong@example.net');
		expect(composeMailAddress('', 'gmail.com')).toBe('');
		expect(composeMailAddress('mohyeong', '')).toBe('');
	});

	test('returns automatic server settings for supported providers', () => {
		expect(mailProviderPreset('gmail')).toMatchObject({
			domain: 'gmail.com',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com'
		});
		expect(mailProviderPreset('custom')).toBe(null);
	});
});
