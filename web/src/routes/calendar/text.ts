import { googleOAuthClientGuideText, type CalendarGoogleOAuthClientGuideText } from './calendar-google-oauth-guide-text';

export type CalendarLocaleText = {
	pageTitle: string;
	title: string;
	subtitle: string;
	work: string;
	previousMonth: string;
	nextMonth: string;
	pickMonthAndYear: string;
	previousYear: string;
	nextYear: string;
	previousTwelveYears: string;
	nextTwelveYears: string;
	subscriptionSettings: string;
	subscriptionReady: string;
	externalCalendarAccount: string;
	accountStatusLoading: string;
	accountStatusLoadFailed: string;
	googleCalendarDisconnected: string;
	googleCalendarConnected: string;
	googleCalendarConnectedTemplate: string;
	googleCalendarReauthRequired: string;
	googleCalendarSelectedCalendarTemplate: string;
	googleCalendarSelectionRequired: string;
	googleCalendarWritePermissionRequired: string;
	googleCalendarInaccessible: string;
	googleCalendarInitialSyncPending: string;
	googleCalendarInitialExportPending: string;
	googleCalendarSyncReady: string;
	googleCalendarConnectAction: string;
	googleCalendarReconnectAction: string;
	googleCalendarSwitchAccountAction: string;
	googleCalendarReconnectHint: string;
	googleCalendarReadyHint: string;
	googleCalendarUnavailableHint: string;
	googleCalendarConnectionComplete: string;
	googleCalendarConnectionFailed: string;
	googleCalendarSelectLabel: string;
	googleCalendarPrimaryLabel: string;
	googleCalendarListLoading: string;
	googleCalendarListEmpty: string;
	googleCalendarListLoadError: string;
	googleCalendarSelectionDisabledWritePermission: string;
	googleCalendarSelectionDisabledUnsupported: string;
	googleCalendarSelectionSaveAction: string;
	googleCalendarSelectionSaving: string;
	googleCalendarSelectionSaveError: string;
	googleOAuthClientUploadTitle: string;
	googleOAuthClientUploadHint: string;
	googleOAuthClientUploadAction: string;
	googleOAuthClientUploading: string;
	googleOAuthClientUploadError: string;
	googleOAuthClientUploadComplete: string;
	googleOAuthClientFileLabel: string;
	googleOAuthClientChooseFile: string;
	googleOAuthClientReplaceTitle: string;
	googleOAuthClientReplaceHint: string;
	googleOAuthClientGuide: CalendarGoogleOAuthClientGuideText;
	shared: string;
	settings: string;
	refresh: string;
	syncTitle: string;
	syncDescription: string;
	caldav: string;
	username: string;
	password: string;
	ics: string;
	rotate: string;
	copy: string;
	loading: string;
	empty: string;
	error: string;
	saveError: string;
	deleteError: string;
	calendarEventVersionConflictError: string;
	calendarDeleteVersionConflictError: string;
	calendarTargetUnavailableError: string;
	deleteUndoMessage: string;
	deleteUndoAction: string;
	allDay: string;
	today: string;
	previous: string;
	next: string;
	search: string;
	searchCalendar: string;
	searchResults: string;
	noResults: string;
	calendarView: string;
	day: string;
	week: string;
	month: string;
	monthMoreButton: string;
	monthMoreAriaLabel: string;
	new: string;
	newEvent: string;
	editEvent: string;
	draftPopover: {
		calendar: string;
		participants: string;
		participantsPlaceholder: string;
		removeParticipantAction: string;
		cancel: string;
		complete: string;
		delete: string;
		startDate: string;
		endDate: string;
		startTime: string;
		endTime: string;
		auditEmpty: string;
		dateTimePicker: {
			previousMonth: string;
			nextMonth: string;
			selectDate: string;
			timeRange: string;
			hour: string;
			minute: string;
			hourSuffix: string;
			minuteSuffix: string;
			save: string;
			editStartDate: string;
			editEndDate: string;
			editStartDateTime: string;
			editEndDateTime: string;
		};
	};
	eventAudit: string;
	eventAuditCreated: string;
	eventAuditUpdated: string;
	conflictBannerTitle: string;
	conflictBannerDescription: string;
	conflictDismiss: string;
	conflictRefresh: string;
	conflictMine: string;
	conflictRemote: string;
	conflictField: Record<string, string>;
};

export type CalendarGoogleAccountText = Pick<
	CalendarLocaleText,
	| 'accountStatusLoading'
	| 'accountStatusLoadFailed'
	| 'googleCalendarDisconnected'
	| 'googleCalendarConnected'
	| 'googleCalendarConnectedTemplate'
	| 'googleCalendarReauthRequired'
	| 'googleCalendarSelectedCalendarTemplate'
	| 'googleCalendarSelectionRequired'
	| 'googleCalendarWritePermissionRequired'
	| 'googleCalendarInaccessible'
	| 'googleCalendarInitialSyncPending'
	| 'googleCalendarInitialExportPending'
	| 'googleCalendarSyncReady'
	| 'googleCalendarConnectAction'
	| 'googleCalendarReconnectAction'
	| 'googleCalendarSwitchAccountAction'
	| 'googleCalendarReconnectHint'
	| 'googleCalendarReadyHint'
	| 'googleCalendarUnavailableHint'
	| 'googleCalendarConnectionComplete'
	| 'googleCalendarSelectLabel'
	| 'googleCalendarPrimaryLabel'
	| 'googleCalendarListLoading'
	| 'googleCalendarListEmpty'
	| 'googleCalendarListLoadError'
	| 'googleCalendarSelectionDisabledWritePermission'
	| 'googleCalendarSelectionDisabledUnsupported'
	| 'googleCalendarSelectionSaveAction'
	| 'googleCalendarSelectionSaving'
	| 'googleCalendarSelectionSaveError'
	| 'googleOAuthClientUploadTitle'
	| 'googleOAuthClientUploadHint'
	| 'googleOAuthClientUploadAction'
	| 'googleOAuthClientUploading'
	| 'googleOAuthClientUploadComplete'
	| 'googleOAuthClientFileLabel'
	| 'googleOAuthClientChooseFile'
	| 'googleOAuthClientReplaceTitle'
	| 'googleOAuthClientReplaceHint'
	| 'googleOAuthClientGuide'
	| 'externalCalendarAccount'
>;

export const calendarText: Record<'ko' | 'en', CalendarLocaleText> = {
	ko: {
		pageTitle: '일정 · 김인턴',
		title: '일정',
		subtitle: '팀 일정과 외부 일정 연동을 한 화면에서 다룹니다.',
		work: '팀 일정',
		previousMonth: '이전 달',
		nextMonth: '다음 달',
		pickMonthAndYear: '월과 연도 선택',
		previousYear: '이전 해',
		nextYear: '다음 해',
		previousTwelveYears: '이전 12년',
		nextTwelveYears: '다음 12년',
		subscriptionSettings: '구독 설정',
		subscriptionReady: 'CalDAV/ICS 구독 URL 준비됨',
		externalCalendarAccount: '연결된 Google 캘린더',
		accountStatusLoading: '연결 상태 확인 중',
		accountStatusLoadFailed: '연결 상태 확인 실패',
		googleCalendarDisconnected: '미연결',
		googleCalendarConnected: '연결됨',
		googleCalendarConnectedTemplate: '연결됨: {email}',
		googleCalendarReauthRequired: '재연결 필요',
		googleCalendarSelectedCalendarTemplate: '사용 중인 캘린더: {calendar}',
		googleCalendarSelectionRequired: '캘린더 선택 필요',
		googleCalendarWritePermissionRequired: '쓰기 권한 필요',
		googleCalendarInaccessible: '캘린더 접근 불가',
		googleCalendarInitialSyncPending: '초기 동기화 중',
		googleCalendarInitialExportPending: '초기 내보내기 중',
		googleCalendarSyncReady: '동기화 가능',
		googleCalendarConnectAction: '연결',
		googleCalendarReconnectAction: '다시 연결',
		googleCalendarSwitchAccountAction: '계정 변경',
		googleCalendarReconnectHint: 'Google 캘린더 동기화를 다시 시작하려면 계정을 다시 연결하세요.',
		googleCalendarReadyHint: 'Google 캘린더를 연결하세요.',
		googleCalendarUnavailableHint: 'Google 캘린더 연동은 아직 준비되지 않았습니다.',
		googleCalendarConnectionComplete: 'Google 캘린더가 연결됐습니다.',
		googleCalendarConnectionFailed: 'Google 캘린더 연결에 실패했습니다.',
		googleCalendarSelectLabel: '사용할 캘린더',
		googleCalendarPrimaryLabel: '기본',
		googleCalendarListLoading: '캘린더 목록 불러오는 중',
		googleCalendarListEmpty: 'Google 캘린더가 없습니다.',
		googleCalendarListLoadError: 'Google 캘린더 목록을 불러오지 못했습니다.',
		googleCalendarSelectionDisabledWritePermission: '쓰기 권한 필요',
		googleCalendarSelectionDisabledUnsupported: '동기화할 수 없는 캘린더',
		googleCalendarSelectionSaveAction: '캘린더 저장',
		googleCalendarSelectionSaving: '저장 중',
		googleCalendarSelectionSaveError: 'Google 캘린더 선택을 저장하지 못했습니다.',
		googleOAuthClientUploadTitle: 'Google OAuth client.json',
		googleOAuthClientUploadHint: '여기에 client.json을 드롭하거나 파일을 선택하세요.',
		googleOAuthClientUploadAction: '업로드',
		googleOAuthClientUploading: '업로드 중',
		googleOAuthClientUploadError: 'client.json을 업로드하지 못했습니다.',
		googleOAuthClientUploadComplete: 'client.json이 업로드됐습니다. Google 계정을 다시 연결하세요.',
		googleOAuthClientFileLabel: 'client.json 파일 선택',
		googleOAuthClientChooseFile: '파일 선택',
		googleOAuthClientReplaceTitle: 'client.json 교체',
		googleOAuthClientReplaceHint:
			'잘못된 프로젝트나 리디렉션 URI로 만든 파일을 올렸다면 새 client.json으로 교체하세요. 교체하면 현재 Google 계정 연결이 해제됩니다.',
		googleOAuthClientGuide: googleOAuthClientGuideText.ko,
		shared: '공유',
		settings: '설정',
		refresh: '새로고침',
		syncTitle: '설정',
		syncDescription: 'CalDAV/ICS 구독 URL을 확인합니다.',
		caldav: 'CalDAV',
		username: '사용자 이름',
		password: '비밀번호',
		ics: 'ICS',
		rotate: '구독 URL 재발급',
		copy: '복사',
		loading: '일정을 불러오는 중...',
		empty: '일정을 드래그하거나 더블클릭해서 새 일정을 만드세요.',
		error: '일정을 불러오지 못했습니다.',
		saveError: '일정을 저장하지 못했습니다.',
		deleteError: '일정을 삭제하지 못했습니다.',
		calendarEventVersionConflictError:
			'다른 곳에서 이 일정이 변경되었습니다. 서버의 최신 내용을 다시 불러왔습니다.',
		calendarDeleteVersionConflictError:
			'다른 곳에서 이 일정이 변경되어 삭제하지 못했습니다. 서버의 최신 내용을 다시 불러왔습니다.',
		calendarTargetUnavailableError:
			'Google 캘린더의 저장 위치를 사용할 수 없습니다. 계정을 다시 연결하거나 쓸 수 있는 캘린더를 선택하세요.',
		deleteUndoMessage: '일정을 삭제했습니다.',
		deleteUndoAction: '실행 취소',
		allDay: '종일',
		today: '오늘',
		previous: '이전',
		next: '다음',
		search: '검색',
		searchCalendar: '일정 검색',
		searchResults: '검색 결과',
		noResults: '결과 없음',
		calendarView: '일정 보기',
		day: '일',
		week: '주',
		month: '월',
		monthMoreButton: '+{count} 더보기',
		monthMoreAriaLabel: '{date}의 숨겨진 일정 {count}개 보기',
		new: '새로 만들기',
		newEvent: '새 일정',
		editEvent: '일정 편집',
		draftPopover: {
			calendar: '캘린더',
			participants: '참여자',
			participantsPlaceholder: '이름으로 검색해 추가',
			removeParticipantAction: '{name} 제거',
			cancel: '취소',
			complete: '완료',
			delete: '삭제',
			startDate: '시작 날짜',
			endDate: '종료 날짜',
			startTime: '시작 시간',
			endTime: '종료 시간',
			auditEmpty: '등록·수정 정보 없음',
			dateTimePicker: {
				previousMonth: '이전 달',
				nextMonth: '다음 달',
				selectDate: '날짜 선택',
				timeRange: '시간대',
				hour: '시',
				minute: '분',
				hourSuffix: '시',
				minuteSuffix: '분',
				save: '저장하기',
				editStartDate: '시작 날짜 수정',
				editEndDate: '종료 날짜 수정',
				editStartDateTime: '시작 날짜 및 시간 수정',
				editEndDateTime: '종료 날짜 및 시간 수정'
			}
		},
		eventAudit: '일정 변경 이력',
		eventAuditCreated: '등록',
		eventAuditUpdated: '수정',
		conflictBannerTitle: '외부에서 변경된 일정이 있습니다',
		conflictBannerDescription:
			'다른 위치(Google 캘린더 등)에서 같은 일정이 동시에 수정되었습니다. 내 변경은 보존되었지만 외부 변경 사항을 확인해주세요.',
		conflictDismiss: '확인',
		conflictRefresh: '서버에서 다시 불러오기',
		conflictMine: '내 값',
		conflictRemote: '외부 값',
		conflictField: {
			title: '제목',
			description: '설명',
			location: '장소',
			start: '시작 시간',
			end: '종료 시간',
			timeZone: '시간대',
			isAllDay: '종일 여부',
			color: '색상',
			participants: '참여자',
			reminderLeadHours: '알림 시간'
		}
	},
	en: {
		pageTitle: 'Calendar · intern kim',
		title: 'Calendar',
		subtitle: 'Team schedule and external calendar sync in one place.',
		work: 'Work',
		previousMonth: 'Previous month',
		nextMonth: 'Next month',
		pickMonthAndYear: 'Pick month and year',
		previousYear: 'Previous year',
		nextYear: 'Next year',
		previousTwelveYears: 'Previous 12 years',
		nextTwelveYears: 'Next 12 years',
		subscriptionSettings: 'Subscription settings',
		subscriptionReady: 'CalDAV/ICS subscription ready',
		externalCalendarAccount: 'Connected Google Calendar',
		accountStatusLoading: 'Checking connection status',
		accountStatusLoadFailed: 'Could not check connection status',
		googleCalendarDisconnected: 'Not connected',
		googleCalendarConnected: 'Connected',
		googleCalendarConnectedTemplate: 'Connected: {email}',
		googleCalendarReauthRequired: 'Reconnect required',
		googleCalendarSelectedCalendarTemplate: 'Using calendar: {calendar}',
		googleCalendarSelectionRequired: 'Calendar selection required',
		googleCalendarWritePermissionRequired: 'Write permission required',
		googleCalendarInaccessible: 'Calendar access unavailable',
		googleCalendarInitialSyncPending: 'Initial sync pending',
		googleCalendarInitialExportPending: 'Initial export pending',
		googleCalendarSyncReady: 'Ready to sync',
		googleCalendarConnectAction: 'Connect',
		googleCalendarReconnectAction: 'Reconnect',
		googleCalendarSwitchAccountAction: 'Change account',
		googleCalendarReconnectHint: 'Reconnect the account to resume Google Calendar sync.',
		googleCalendarReadyHint: 'Connect Google Calendar.',
		googleCalendarUnavailableHint: 'Google Calendar sync is not ready yet.',
		googleCalendarConnectionComplete: 'Google Calendar is connected.',
		googleCalendarConnectionFailed: 'Google Calendar connection failed.',
		googleCalendarSelectLabel: 'Calendar to use',
		googleCalendarPrimaryLabel: 'Primary',
		googleCalendarListLoading: 'Loading calendar list',
		googleCalendarListEmpty: 'No Google calendars are available.',
		googleCalendarListLoadError: 'Could not load Google calendars.',
		googleCalendarSelectionDisabledWritePermission: 'Write permission required',
		googleCalendarSelectionDisabledUnsupported: 'Cannot sync this calendar',
		googleCalendarSelectionSaveAction: 'Save calendar',
		googleCalendarSelectionSaving: 'Saving',
		googleCalendarSelectionSaveError: 'Could not save the Google Calendar selection.',
		googleOAuthClientUploadTitle: 'Google OAuth client.json',
		googleOAuthClientUploadHint: 'Drop client.json here or choose the file.',
		googleOAuthClientUploadAction: 'Upload',
		googleOAuthClientUploading: 'Uploading',
		googleOAuthClientUploadError: 'Could not upload client.json.',
		googleOAuthClientUploadComplete: 'client.json was uploaded. Reconnect the Google account.',
		googleOAuthClientFileLabel: 'Choose client.json file',
		googleOAuthClientChooseFile: 'Choose file',
		googleOAuthClientReplaceTitle: 'Replace client.json',
		googleOAuthClientReplaceHint:
			'If the uploaded file came from the wrong project or redirect URI, replace it with a new client.json. Replacing it disconnects the current Google account.',
		googleOAuthClientGuide: googleOAuthClientGuideText.en,
		shared: 'Shared',
		settings: 'Settings',
		refresh: 'Refresh',
		syncTitle: 'Settings',
		syncDescription: 'Review CalDAV/ICS subscription URLs.',
		caldav: 'CalDAV',
		username: 'Username',
		password: 'Password',
		ics: 'ICS',
		rotate: 'Rotate subscription URL',
		copy: 'Copy',
		loading: 'Loading calendar...',
		empty: 'Drag or click the calendar to create an event.',
		error: 'Could not load the calendar.',
		saveError: 'Could not save the event.',
		deleteError: 'Could not delete the event.',
		calendarEventVersionConflictError:
			'This event changed elsewhere. The latest server version has been reloaded.',
		calendarDeleteVersionConflictError:
			'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.',
		calendarTargetUnavailableError:
			'The Google Calendar destination is unavailable. Reconnect the account or select a writable calendar.',
		deleteUndoMessage: 'Event deleted.',
		deleteUndoAction: 'Undo',
		allDay: 'All day',
		today: 'Today',
		previous: 'Previous',
		next: 'Next',
		search: 'Search',
		searchCalendar: 'Search calendar',
		searchResults: 'Search results',
		noResults: 'No results',
		calendarView: 'Calendar view',
		day: 'Day',
		week: 'Week',
		month: 'Month',
		monthMoreButton: '+{count} more',
		monthMoreAriaLabel: 'Show {count} more events on {date}',
		new: 'New',
		newEvent: 'New Event',
		editEvent: 'Edit Event',
		draftPopover: {
			calendar: 'Calendar',
			participants: 'Participants',
			participantsPlaceholder: 'Search by name to add',
			removeParticipantAction: 'Remove {name}',
			cancel: 'Cancel',
			complete: 'Done',
			delete: 'Delete',
			startDate: 'Start date',
			endDate: 'End date',
			startTime: 'Start time',
			endTime: 'End time',
			auditEmpty: 'No creation or edit details',
			dateTimePicker: {
				previousMonth: 'Previous month',
				nextMonth: 'Next month',
				selectDate: 'Select date',
				timeRange: 'Time',
				hour: 'Hour',
				minute: 'Minute',
				hourSuffix: 'h',
				minuteSuffix: 'min',
				save: 'Save',
				editStartDate: 'Edit start date',
				editEndDate: 'Edit end date',
				editStartDateTime: 'Edit start date and time',
				editEndDateTime: 'Edit end date and time'
			}
		},
		eventAudit: 'Event audit',
		eventAuditCreated: 'Created',
		eventAuditUpdated: 'Updated',
		conflictBannerTitle: 'External changes detected',
		conflictBannerDescription:
			'The same events were modified elsewhere (e.g. Google Calendar) at the same time. Your changes are preserved, but please review the external edits.',
		conflictDismiss: 'Acknowledge',
		conflictRefresh: 'Reload from server',
		conflictMine: 'Yours',
		conflictRemote: 'Remote',
		conflictField: {
			title: 'Title',
			description: 'Description',
			location: 'Location',
			start: 'Start time',
			end: 'End time',
			timeZone: 'Time zone',
			isAllDay: 'All day',
			color: 'Color',
			participants: 'Participants',
			reminderLeadHours: 'Reminder lead'
		}
	}
};
