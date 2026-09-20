import { describe, expect, test } from 'bun:test';

import { mailText } from '../../../src/routes/mail/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('mailText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(mailText.en).sort()).toEqual(collectTextShape(mailText.ko).sort());
	});

	test('calls a mail a 메일, leaving 메시지 to the messenger', () => {
		const messengerWording = collectKoreanStrings(mailText.ko).filter((entry) => entry.value.includes('메시지'));

		expect(messengerWording).toEqual([]);
	});

	test('uses beginner-friendly provider setup copy', () => {
		expect(mailText.ko.providers.gmail).toBe('gmail.com');
		expect(mailText.ko.providers.naver).toBe('naver.com');
		expect(mailText.ko.providers.daum).toBe('daum.net');
		expect(mailText.ko.providers.hanmail).toBe('hanmail.net');
		expect(mailText.ko.checkingMail).toBe('메일을 확인하는 중...');
		expect(mailText.ko.loadingMessages).toBe('메일을 불러오는 중...');
		expect(mailText.ko.settingsSheet.advancedSettings).toBe('고급 설정');
		expect(mailText.ko.settingsSheet.googleAppPasswordLink).toBe('Google 앱 비밀번호 만들기');
		expect(mailText.ko.settingsSheet.googleAccountNote).toBe('먼저 올바른 Google 계정으로 로그인했는지 확인하세요. 다른 계정에서 만든 앱 비밀번호는 연결되지 않습니다.');
		expect(mailText.ko.settingsSheet.googleSetupNote).toBe('생성된 앱 비밀번호는 이 화면을 닫으면 다시 볼 수 없습니다. 안전한 곳에 저장한 뒤, 앱 비밀번호 칸에 붙여넣으세요.');
		expect(mailText.ko.settingsSheet.naverSetupSteps).toEqual(['IMAP/SMTP 사용 설정하기', '애플리케이션 비밀번호 생성하기']);
		expect(mailText.ko.settingsSheet.naverAccountNote).toBe('먼저 올바른 네이버 계정으로 로그인했는지 확인하세요. 다른 계정에서 만든 애플리케이션 비밀번호는 연결되지 않습니다.');
		expect(mailText.ko.settingsSheet.naverSetupNotes).toEqual(['IMAP/SMTP 사용을 사용함으로 바꾸고 저장하세요. IMAP 동기화 메일 제한은 한 번에 가져올 최대 메일 수입니다.', '생성된 애플리케이션 비밀번호는 이 화면을 닫으면 다시 볼 수 없습니다. 안전한 곳에 저장한 뒤, 앱 비밀번호 칸에 붙여넣으세요.']);
		expect(mailText.ko.settingsSheet.daumSetupLink).toBe('Daum/Hanmail 설정 링크 보기');
		expect(mailText.ko.settingsSheet.daumSetupSteps).toEqual(['IMAP/SMTP 사용 설정하기', 'Daum 앱 비밀번호 생성하기']);
		expect(mailText.ko.settingsSheet.daumSetupNotes).toEqual(['IMAP/SMTP 사용을 사용함으로 바꾸고 저장하세요. 설정 화면의 아이디를 IMAP/SMTP 로그인 계정으로 사용합니다.', '생성된 앱 비밀번호는 앱 비밀번호 칸에 붙여넣으세요. 앱 비밀번호는 다시 확인하기 어려우니 안전한 곳에 저장하세요.']);
		expect(mailText.ko.settingsSheet.setupImageAlts.daumAppPassword).toBe('Daum 앱 비밀번호 생성 예시');
		expect(mailText.en.checkingMail).toBe('Checking mail...');
		expect(mailText.en.loadingMessages).toBe('Loading messages...');
		expect(mailText.en.settingsSheet.advancedSettings).toBe('Advanced settings');
		expect(mailText.en.settingsSheet.googleAppPasswordLink).toBe('Create Google app password');
		expect(mailText.en.settingsSheet.googleAccountNote).toBe('First check that you are signed in to the correct Google account. App passwords from another account will not connect.');
		expect(mailText.en.settingsSheet.googleSetupNote).toBe('You cannot view the generated app password again after closing this screen. Save it somewhere safe, then paste it into the app password field.');
		expect(mailText.en.settingsSheet.naverSetupSteps).toEqual(['Enable IMAP/SMTP', 'Create app password']);
		expect(mailText.en.settingsSheet.naverAccountNote).toBe('First check that you are signed in to the correct Naver account. App passwords from another account will not connect.');
		expect(mailText.en.settingsSheet.naverSetupNotes).toEqual(['Turn IMAP/SMTP on, then save the setting. The IMAP sync mail limit is the maximum number of messages to fetch at once.', 'You cannot view the generated app password again after closing this screen. Save it somewhere safe, then paste it into the app password field.']);
		expect(mailText.en.settingsSheet.daumSetupLink).toBe('Show Daum/Hanmail setup links');
		expect(mailText.en.settingsSheet.daumSetupSteps).toEqual(['Enable IMAP/SMTP', 'Create Daum app password']);
		expect(mailText.en.settingsSheet.daumSetupNotes).toEqual(['Turn IMAP/SMTP on, then save the setting. Use the ID shown on that settings screen as the IMAP/SMTP login account.', 'Paste the generated app password into the app password field. Save it somewhere safe because it may be hard to view again.']);
		expect(mailText.en.settingsSheet.setupImageAlts.daumAppPassword).toBe('Daum app password creation example');
	});
});

function collectKoreanStrings(node: TextNode, path: string[] = []): { path: string; value: string }[] {
	if (typeof node === 'string') return [{ path: path.join('.'), value: node }];
	if (Array.isArray(node)) return node.flatMap((entry, index) => collectKoreanStrings(entry, [...path, String(index)]));

	return Object.entries(node).flatMap(([key, value]) => collectKoreanStrings(value, [...path, key]));
}

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
