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
		expect(mailProviderIDFromDomain('daum.net')).toBe('daum');
		expect(mailProviderIDFromDomain('HANMAIL.NET')).toBe('hanmail');
		expect(mailProviderIDFromDomain('example.com')).toBe('custom');
	});

	test('builds email address draft from full addresses', () => {
		expect(mailAddressDraftFromEmail('')).toEqual({
			localPart: '',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail('gyeonbon')).toEqual({
			localPart: 'gyeonbon',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail(' gyeonbon@gmail.com ')).toEqual({
			localPart: 'gyeonbon',
			providerID: 'gmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail('chanee234@hanmail.net')).toEqual({
			localPart: 'chanee234',
			providerID: 'hanmail',
			customDomain: ''
		});
		expect(mailAddressDraftFromEmail('staff@example.com')).toEqual({
			localPart: 'staff',
			providerID: 'custom',
			customDomain: 'example.com'
		});
	});

	test('composes normalized mail addresses', () => {
		expect(composeMailAddress(' gyeonbon ', ' Gmail.COM ')).toBe('gyeonbon@gmail.com');
		expect(composeMailAddress('', 'gmail.com')).toBe('');
		expect(composeMailAddress('gyeonbon', '')).toBe('');
	});

	test('returns automatic server settings for supported providers', () => {
		expect(mailProviderPreset('gmail')).toMatchObject({
			domain: 'gmail.com',
			imapHost: 'imap.gmail.com',
			smtpHost: 'smtp.gmail.com'
		});
		expect(mailProviderPreset('daum')).toMatchObject({
			domain: 'daum.net',
			imapHost: 'imap.daum.net',
			imapPort: 993,
			imapSecurity: 'tls',
			smtpHost: 'smtp.daum.net',
			smtpPort: 465,
			smtpSecurity: 'tls',
			loginAccountMode: 'localPart'
		});
		expect(mailProviderPreset('hanmail')).toMatchObject({
			domain: 'hanmail.net',
			imapHost: 'imap.daum.net',
			smtpHost: 'smtp.daum.net',
			loginAccountMode: 'localPart'
		});
		expect(mailProviderPreset('custom')).toBe(null);
	});
});
