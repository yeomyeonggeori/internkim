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
		expect(calendarText.en.googleCalendarSelectedCalendarTemplate).toBe('Using calendar: {calendar}');
		expect(calendarText.en.googleCalendarSelectionRequired).toBe('Calendar selection required');
		expect(calendarText.en.googleCalendarWritePermissionRequired).toBe('Write permission required');
		expect(calendarText.en.googleCalendarInaccessible).toBe('Calendar access unavailable');
		expect(calendarText.en.googleCalendarInitialSyncPending).toBe('Initial sync pending');
		expect(calendarText.en.googleCalendarSyncReady).toBe('Ready to sync');
		expect(calendarText.en.googleCalendarConnectAction).toBe('Connect');
		expect(calendarText.en.googleCalendarReconnectAction).toBe('Reconnect');
		expect(calendarText.en.googleCalendarSwitchAccountAction).toBe('Change account');
		expect(calendarText.en.googleCalendarReconnectHint).toBe('Reconnect the account to resume Google Calendar sync.');
		expect(calendarText.en.googleCalendarReadyHint).toBe('Connect Google Calendar.');
		expect(calendarText.en.googleCalendarUnavailableHint).toBe('Google Calendar sync is not ready yet.');
		expect(calendarText.en.googleCalendarConnectionComplete).toBe('Google Calendar is connected.');
		expect(calendarText.en.googleCalendarConnectionFailed).toBe('Google Calendar connection failed.');
		expect(calendarText.en.googleCalendarSelectLabel).toBe('Calendar to use');
		expect(calendarText.en.googleCalendarListEmpty).toBe('No Google calendars are available.');
		expect(calendarText.en.googleCalendarSelectionDisabledWritePermission).toBe('Write permission required');
		expect(calendarText.en.googleCalendarSelectionDisabledUnsupported).toBe('Cannot sync this calendar');
		expect(calendarText.en.googleCalendarSelectionSaveAction).toBe('Save calendar');
		expect(calendarText.en.googleOAuthClientUploadTitle).toBe('Google OAuth client.json');
		expect(calendarText.en.googleOAuthClientUploadHint).toBe('Drop client.json here or choose the file.');
		expect(calendarText.en.googleOAuthClientUploadAction).toBe('Upload');
		expect(calendarText.en.googleOAuthClientChooseFile).toBe('Choose file');
		expect(calendarText.en.googleOAuthClientReplaceTitle).toBe('Replace client.json');
		expect(calendarText.en.googleOAuthClientReplaceHint.includes('wrong project or redirect URI')).toBe(true);
		expect(calendarText.en.googleOAuthClientReplaceHint.includes('disconnects the current Google account')).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.title).toBe('How to create client.json');
		expect(calendarText.en.googleOAuthClientGuide.checks.includes(
			'If you only connect a personal calendar or a personally owned calendar, you can create client.json in a Google Cloud project you can manage.'
		)).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.checks.includes(
			'If you connect a team or company shared calendar, sign in to Google Cloud with the company admin account and create client.json in an organization-owned project.'
		)).toBe(true);
		expect(calendarText.en.googleOAuthClientGuide.checks.includes(
			'Connect Google Calendar with a Google account that has access and write permission for the selected calendar.'
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
		expect(calendarText.ko.googleCalendarSelectedCalendarTemplate).toBe('사용 중인 캘린더: {calendar}');
		expect(calendarText.ko.googleCalendarSelectionRequired).toBe('캘린더 선택 필요');
		expect(calendarText.ko.googleCalendarWritePermissionRequired).toBe('쓰기 권한 필요');
		expect(calendarText.ko.googleCalendarInaccessible).toBe('캘린더 접근 불가');
		expect(calendarText.ko.googleCalendarInitialSyncPending).toBe('초기 동기화 중');
		expect(calendarText.ko.googleCalendarSyncReady).toBe('동기화 가능');
		expect(calendarText.ko.googleCalendarConnectAction).toBe('연결');
		expect(calendarText.ko.googleCalendarReconnectAction).toBe('다시 연결');
		expect(calendarText.ko.googleCalendarSwitchAccountAction).toBe('계정 변경');
		expect(calendarText.ko.googleCalendarReconnectHint).toBe('Google 캘린더 동기화를 다시 시작하려면 계정을 다시 연결하세요.');
		expect(calendarText.ko.googleCalendarReadyHint).toBe('Google 캘린더를 연결하세요.');
		expect(calendarText.ko.googleCalendarUnavailableHint).toBe('Google 캘린더 연동은 아직 준비되지 않았습니다.');
		expect(calendarText.ko.googleCalendarConnectionComplete).toBe('Google 캘린더가 연결됐습니다.');
		expect(calendarText.ko.googleCalendarConnectionFailed).toBe('Google 캘린더 연결에 실패했습니다.');
		expect(calendarText.ko.googleCalendarSelectLabel).toBe('사용할 캘린더');
		expect(calendarText.ko.googleCalendarListEmpty).toBe('Google 캘린더가 없습니다.');
		expect(calendarText.ko.googleCalendarSelectionDisabledWritePermission).toBe('쓰기 권한 필요');
		expect(calendarText.ko.googleCalendarSelectionDisabledUnsupported).toBe('동기화할 수 없는 캘린더');
		expect(calendarText.ko.googleCalendarSelectionSaveAction).toBe('캘린더 저장');
		expect(calendarText.ko.googleOAuthClientUploadTitle).toBe('Google OAuth client.json');
		expect(calendarText.ko.googleOAuthClientUploadHint).toBe('여기에 client.json을 드롭하거나 파일을 선택하세요.');
		expect(calendarText.ko.googleOAuthClientUploadAction).toBe('업로드');
		expect(calendarText.ko.googleOAuthClientChooseFile).toBe('파일 선택');
		expect(calendarText.ko.googleOAuthClientReplaceTitle).toBe('client.json 교체');
		expect(calendarText.ko.googleOAuthClientReplaceHint.includes('잘못된 프로젝트나 리디렉션 URI')).toBe(true);
		expect(calendarText.ko.googleOAuthClientReplaceHint.includes('현재 Google 계정 연결이 해제')).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.title).toBe('client.json 만드는 방법');
		expect(calendarText.ko.googleOAuthClientGuide.checks.includes(
			'개인 캘린더나 개인 소유 캘린더만 연결할 경우, 관리 가능한 Google Cloud 프로젝트에서 client.json을 만들어도 됩니다.'
		)).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.checks.includes(
			'팀/회사 공유 캘린더를 연결할 경우, 회사 관리자 계정으로 Google Cloud에 로그인하고 조직 소유 프로젝트에서 client.json을 만드세요.'
		)).toBe(true);
		expect(calendarText.ko.googleOAuthClientGuide.checks.includes(
			'캘린더 연결은 선택할 캘린더에 접근 및 쓰기 권한이 있는 Google 계정으로 진행하세요.'
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

	test('localizes an unavailable calendar target separately from generic persistence errors', () => {
		expect(calendarText.ko.calendarTargetUnavailableError).toBe(
			'Google 캘린더의 저장 위치를 사용할 수 없습니다. 계정을 다시 연결하거나 쓸 수 있는 캘린더를 선택하세요.'
		);
		expect(calendarText.en.calendarTargetUnavailableError).toBe(
			'The Google Calendar destination is unavailable. Reconnect the account or select a writable calendar.'
		);
	});
});
