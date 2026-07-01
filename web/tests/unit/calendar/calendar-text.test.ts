import { describe, expect, test } from 'bun:test';
import { calendarText } from '../../../src/routes/calendar/text';

describe('calendar text', () => {
	test('separates subscription settings from external account status', () => {
		expect(calendarText.en.subscriptionSettings).toBe('Subscription settings');
		expect(calendarText.en.subscriptionReady).toBe('CalDAV/ICS subscription ready');
		expect(calendarText.en.settings).toBe('Settings');
		expect(calendarText.en.externalCalendarAccount).toBe('Connected Google Calendar');
		expect(calendarText.en.googleCalendarDisconnected).toBe('Not connected');
		expect(calendarText.en.googleCalendarConnectedTemplate).toBe('Connected: {email}');
		expect(calendarText.en.googleCalendarConnectAction).toBe('Connect');
		expect(calendarText.en.googleCalendarReconnectAction).toBe('Reconnect');
		expect(calendarText.en.googleCalendarReconnectHint).toBe('Reconnect the account to resume Google Calendar sync.');
		expect(calendarText.en.googleCalendarReadyHint).toBe('Connect Google Calendar.');
		expect(calendarText.en.googleCalendarUnavailableHint).toBe('Google Calendar sync is not ready yet.');
		expect(calendarText.en.googleOAuthClientUploadTitle).toBe('Google OAuth client.json');
		expect(calendarText.en.googleOAuthClientUploadHint).toBe('Drop client.json here or choose the file.');
		expect(calendarText.en.googleOAuthClientUploadAction).toBe('Upload');
		expect(calendarText.en.googleOAuthClientChooseFile).toBe('Choose file');
		expect(calendarText.en.googleOAuthClientReplaceTitle).toBe('Replace client.json');
		expect(calendarText.en.googleOAuthClientReplaceHint.includes('wrong project or redirect URI')).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.title).toBe('How to create client.json');
		expect(calendarText.en.googleOAuthClientGuide.checks.includes(
			'Confirm the Google Cloud profile in the top right is the company admin account.'
		)).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.checks.includes(
			'Confirm the project is owned by the company organization, not a personal account.'
		)).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.checks.join(' ').includes('dawn.kim')).toBe(false);
		expect(calendarText.en.googleOAuthClientGuide.steps[0]?.action).toEqual({
			label: 'Open project creation',
			url: 'https://console.cloud.google.com/projectcreate'
		});
		expect(calendarText.en.googleOAuthClientGuide.steps[1]?.title).toBe('Enable the CalDAV API.');
		expect(calendarText.en.googleOAuthClientGuide.steps[1]?.action).toEqual({
			label: 'Open CalDAV API',
			url: 'https://console.cloud.google.com/marketplace/product/google/caldav.googleapis.com'
		});
		expect(calendarText.en.googleOAuthClientGuide.steps[3]?.action).toEqual({
			label: 'Open OAuth clients',
			url: 'https://console.cloud.google.com/auth/clients'
		});
		expect(calendarText.en.googleOAuthClientGuide.steps[5]?.body.includes('Save the password somewhere secure')).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.javascriptOrigin).toBe('Authorized JavaScript origin');
		expect(calendarText.en.googleOAuthClientGuide.steps.length).toBe(7);
		expect(calendarText.ko.subscriptionSettings).toBe('구독 설정');
		expect(calendarText.ko.subscriptionReady).toBe('CalDAV/ICS 구독 URL 준비됨');
		expect(calendarText.ko.settings).toBe('설정');
		expect(calendarText.ko.externalCalendarAccount).toBe('연결된 Google 캘린더');
		expect(calendarText.ko.googleCalendarDisconnected).toBe('미연결');
		expect(calendarText.ko.googleCalendarConnectedTemplate).toBe('연결됨: {email}');
		expect(calendarText.ko.googleCalendarConnectAction).toBe('연결');
		expect(calendarText.ko.googleCalendarReconnectAction).toBe('다시 연결');
		expect(calendarText.ko.googleCalendarReconnectHint).toBe('Google 캘린더 동기화를 다시 시작하려면 계정을 다시 연결하세요.');
		expect(calendarText.ko.googleCalendarReadyHint).toBe('Google 캘린더를 연결하세요.');
		expect(calendarText.ko.googleCalendarUnavailableHint).toBe('Google 캘린더 연동은 아직 준비되지 않았습니다.');
		expect(calendarText.ko.googleOAuthClientUploadTitle).toBe('Google OAuth client.json');
		expect(calendarText.ko.googleOAuthClientUploadHint).toBe('여기에 client.json을 드롭하거나 파일을 선택하세요.');
		expect(calendarText.ko.googleOAuthClientUploadAction).toBe('업로드');
		expect(calendarText.ko.googleOAuthClientChooseFile).toBe('파일 선택');
		expect(calendarText.ko.googleOAuthClientReplaceTitle).toBe('client.json 교체');
		expect(calendarText.ko.googleOAuthClientReplaceHint.includes('잘못된 프로젝트나 리디렉션 URI')).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.title).toBe('client.json 만드는 방법');
		expect(calendarText.ko.googleOAuthClientGuide.checks.includes(
			'Google Cloud 오른쪽 위 프로필이 회사 관리자 계정인지 확인하세요.'
		)).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.checks.includes(
			'프로젝트 소유자가 개인 계정이 아니라 회사 조직인지 확인하세요.'
		)).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.checks.join(' ').includes('dawn.kim')).toBe(false);
		expect(calendarText.ko.googleOAuthClientGuide.steps[0]?.action).toEqual({
			label: '프로젝트 만들기',
			url: 'https://console.cloud.google.com/projectcreate'
		});
		expect(calendarText.ko.googleOAuthClientGuide.steps[1]?.title).toBe('CalDAV API를 사용 설정합니다.');
		expect(calendarText.ko.googleOAuthClientGuide.steps[1]?.action).toEqual({
			label: 'CalDAV API 활성화 열기',
			url: 'https://console.cloud.google.com/marketplace/product/google/caldav.googleapis.com'
		});
		expect(calendarText.ko.googleOAuthClientGuide.steps[3]?.action).toEqual({
			label: 'OAuth 클라이언트 열기',
			url: 'https://console.cloud.google.com/auth/clients'
		});
		expect(calendarText.ko.googleOAuthClientGuide.steps[5]?.body.includes('비밀번호가 표시되면 안전한 곳에 따로 저장하세요')).toBe(
			true
		);
		expect(calendarText.ko.googleOAuthClientGuide.javascriptOrigin).toBe('승인된 JavaScript 원본');
		expect(calendarText.ko.googleOAuthClientGuide.steps.length).toBe(7);
	});

	test('localizes event audit labels', () => {
		expect(calendarText.en.eventAuditCreated).toBe('Created');
		expect(calendarText.en.eventAuditUpdated).toBe('Updated');
		expect(calendarText.ko.eventAuditCreated).toBe('등록');
		expect(calendarText.ko.eventAuditUpdated).toBe('수정');
	});
});
