import { describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('mailText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(mailText.en).sort()).toEqual(collectTextShape(mailText.ko).sort());
	});

	test('uses beginner-friendly provider setup copy', () => {
		expect(mailText.ko.providers.gmail).toBe('gmail.com');
		expect(mailText.ko.providers.naver).toBe('naver.com');
		expect(mailText.ko.checkingMail).toBe('메일을 확인하는 중...');
		expect(mailText.ko.loadingMessages).toBe('메시지를 불러오는 중...');
		expect(mailText.ko.settingsSheet.advancedSettings).toBe('고급 설정');
		expect(mailText.ko.settingsSheet.googleAppPasswordLink).toBe('Google 앱 비밀번호 만들기');
		expect(mailText.en.checkingMail).toBe('Checking mail...');
		expect(mailText.en.loadingMessages).toBe('Loading messages...');
		expect(mailText.en.settingsSheet.advancedSettings).toBe('Advanced settings');
		expect(mailText.en.settingsSheet.googleAppPasswordLink).toBe('Create Google app password');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
