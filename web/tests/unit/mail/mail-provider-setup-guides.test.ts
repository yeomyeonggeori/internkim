import { describe, expect, test } from 'bun:test';

import { mailProviderSetupGuide } from '../../../src/routes/mail/mail-provider-setup-guides';
import { mailText } from '../../../src/routes/mail/text';

describe('mail provider setup guides', () => {
	test('returns Google app password guide with localized image alt text', () => {
		expect(mailProviderSetupGuide('gmail', mailText.ko)).toMatchObject({
			triggerLabel: 'Google 앱 비밀번호 만들기',
			isNumbered: false,
			items: [
				{
					href: 'https://myaccount.google.com/apppasswords',
					image: {
						src: '/mail/providers/google/app-password-created-example.png',
						alt: 'Google 앱 비밀번호 생성 완료 예시'
					}
				}
			]
		});
		expect(mailProviderSetupGuide('gmail', mailText.en)?.items[0]?.image.alt).toBe('Google app password creation example');
	});

	test('returns Naver guide links and images in setup order', () => {
		const guide = mailProviderSetupGuide('naver', mailText.ko);

		expect(guide?.items.map((item) => item.href)).toEqual([
			'https://mail.naver.com/v2/settings/smtp/imap',
			'https://nid.naver.com/user2/help/2StepVerif'
		]);
		expect(guide?.items.map((item) => item.image.src)).toEqual([
			'/mail/providers/naver/imap-smtp-example.png',
			'/mail/providers/naver/app-password-example.png'
		]);
	});

	test('shares Daum guide links and images for Hanmail', () => {
		const daumGuide = mailProviderSetupGuide('daum', mailText.ko);
		const hanmailGuide = mailProviderSetupGuide('hanmail', mailText.ko);

		expect(daumGuide?.items.map((item) => item.href)).toEqual([
			'https://mail.daum.net/setting/POP3IMAP',
			'https://member.daum.net/my/security'
		]);
		expect(hanmailGuide).toEqual(daumGuide);
		expect(daumGuide?.items.map((item) => item.image.src)).toEqual([
			'/mail/providers/daum/imap-smtp-example.png',
			'/mail/providers/daum/app-password-example.png'
		]);
	});

	test('does not return a setup guide for custom providers', () => {
		expect(mailProviderSetupGuide('custom', mailText.ko)).toBe(null);
	});
});
