type CalendarLocaleText = {
	pageTitle: string;
	title: string;
	subtitle: string;
	work: string;
	myCalendars: string;
	addCalendar: string;
	moreOptions: string;
	calendarVisibility: string;
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
	shared: string;
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
	new: string;
	newEvent: string;
	eventAudit: string;
	conflictBannerTitle: string;
	conflictBannerDescription: string;
	conflictDismiss: string;
	conflictRefresh: string;
	conflictMine: string;
	conflictRemote: string;
	conflictField: Record<string, string>;
};

export const calendarText: Record<'ko' | 'en', CalendarLocaleText> = {
	ko: {
		pageTitle: '일정 · 김인턴',
		title: '일정',
		subtitle: '팀 일정과 외부 일정 연동을 한 화면에서 다룹니다.',
		work: '팀 일정',
		myCalendars: '내 일정',
		addCalendar: '일정 추가',
		moreOptions: '일정 옵션 더보기',
		calendarVisibility: '표시 여부',
		previousMonth: '이전 달',
		nextMonth: '다음 달',
		pickMonthAndYear: '월과 연도 선택',
		previousYear: '이전 해',
		nextYear: '다음 해',
		previousTwelveYears: '이전 12년',
		nextTwelveYears: '다음 12년',
		subscriptionSettings: '구독 설정',
		subscriptionReady: 'CalDAV/ICS 구독 URL 준비됨',
		externalCalendarAccount: '외부 캘린더 계정',
		accountStatusLoading: '연결 상태 확인 중',
		accountStatusLoadFailed: '연결 상태 확인 실패',
		googleCalendarDisconnected: 'Google Calendar 미연결',
		googleCalendarConnected: 'Google Calendar 연결됨',
		googleCalendarConnectedTemplate: 'Google Calendar 연결됨: {email}',
		googleCalendarReauthRequired: 'Google Calendar 재인증 필요',
		shared: '공유',
		refresh: '새로고침',
		syncTitle: '연동',
		syncDescription: 'CalDAV/ICS 구독 URL과 외부 캘린더 계정 상태를 확인합니다.',
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
		new: '새로 만들기',
		newEvent: '새 일정',
		eventAudit: '일정 변경 이력',
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
			reminderLeadHours: '알림 시간'
		}
	},
	en: {
		pageTitle: 'Calendar · intern kim',
		title: 'Calendar',
		subtitle: 'Team schedule and external calendar sync in one place.',
		work: 'Work',
		myCalendars: 'My calendars',
		addCalendar: 'Add calendar',
		moreOptions: 'More calendar options',
		calendarVisibility: 'calendar visibility',
		previousMonth: 'Previous month',
		nextMonth: 'Next month',
		pickMonthAndYear: 'Pick month and year',
		previousYear: 'Previous year',
		nextYear: 'Next year',
		previousTwelveYears: 'Previous 12 years',
		nextTwelveYears: 'Next 12 years',
		subscriptionSettings: 'Subscription settings',
		subscriptionReady: 'CalDAV/ICS subscription ready',
		externalCalendarAccount: 'External calendar account',
		accountStatusLoading: 'Checking connection status',
		accountStatusLoadFailed: 'Could not check connection status',
		googleCalendarDisconnected: 'Google Calendar not connected',
		googleCalendarConnected: 'Google Calendar connected',
		googleCalendarConnectedTemplate: 'Google Calendar connected: {email}',
		googleCalendarReauthRequired: 'Google Calendar needs reauthorization',
		shared: 'Shared',
		refresh: 'Refresh',
		syncTitle: 'Sync',
		syncDescription: 'Review CalDAV/ICS subscription URLs and external calendar account status.',
		caldav: 'CalDAV',
		username: 'Username',
		password: 'Password',
		ics: 'ICS',
		rotate: 'Rotate subscription URL',
		copy: 'Copy',
		loading: 'Loading calendar...',
		empty: 'Drag or double-click the calendar to create an event.',
		error: 'Could not load the calendar.',
		saveError: 'Could not save the event.',
		deleteError: 'Could not delete the event.',
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
		new: 'New',
		newEvent: 'New Event',
		eventAudit: 'Event audit',
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
			reminderLeadHours: 'Reminder lead'
		}
	}
};
