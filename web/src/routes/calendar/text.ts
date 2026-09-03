
export type CalendarLocaleText = {
	pageTitle: string;
	title: string;
	subtitle: string;
	work: string;
	holidays: string;
	previousMonth: string;
	nextMonth: string;
	previousWeek: string;
	nextWeek: string;
	previousDay: string;
	nextDay: string;
	pickMonthAndYear: string;
	previousYear: string;
	nextYear: string;
	previousTwelveYears: string;
	nextTwelveYears: string;
	subscriptionSettings: string;
	subscriptionReady: string;
	subscriptionShownOnce: string;
	shared: string;
	settings: string;
	refresh: string;
	syncTitle: string;
	syncDescription: string;
	ics: string;
	rotate: string;
	copy: string;
	loading: string;
	empty: string;
	error: string;
	holidayLoadError: string;
	saveError: string;
	deleteError: string;
	calendarEventVersionConflictError: string;
	calendarDeleteVersionConflictError: string;
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
	filterParticipant: string;
	allParticipants: string;
	noDayEvents: string;
	monthMoreAriaLabel: string;
	new: string;
	newEvent: string;
	editEvent: string;
	deleteEvent: string;
	duplicateEvent: string;
	addEventOnDay: string;
	openDay: string;
	draftPopover: {
		calendar: string;
		participants: string;
		participantsPlaceholder: string;
		participantsSummary: string;
		participantsEmpty: string;
		cancel: string;
		complete: string;
		delete: string;
		startDate: string;
		endDate: string;
		startTime: string;
		endTime: string;
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
	conflictField: Record<string, string>;
};

export const calendarText: Record<'ko' | 'en', CalendarLocaleText> = {
	ko: {
		pageTitle: '일정 · 김인턴',
		title: '일정',
		subtitle: '팀 일정과 외부 일정 연동을 한 화면에서 다룹니다.',
		work: '팀 일정',
		holidays: '공휴일',
		previousMonth: '이전 달',
		nextMonth: '다음 달',
		previousWeek: '지난 주',
		nextWeek: '다음 주',
		previousDay: '어제',
		nextDay: '내일',
		pickMonthAndYear: '월과 연도 선택',
		previousYear: '이전 해',
		nextYear: '다음 해',
		previousTwelveYears: '이전 12년',
		nextTwelveYears: '다음 12년',
		subscriptionSettings: '구독 설정',
		subscriptionReady: '구독 URL 준비됨',
		subscriptionShownOnce: '구독 URL은 만들 때 한 번만 보여 줍니다. 잃어버렸다면 새로 발급하세요.',
		shared: '공유',
		settings: '설정',
		refresh: '새로고침',
		syncTitle: '설정',
		syncDescription: '캘린더 앱에서 구독할 URL을 확인합니다.',
		ics: 'ICS',
		rotate: '구독 URL 재발급',
		copy: '복사',
		loading: '일정을 불러오는 중...',
		empty: '일정을 드래그하거나 더블클릭해서 새 일정을 만드세요.',
		error: '일정을 불러오지 못했습니다.',
		holidayLoadError: '공휴일을 불러오지 못했습니다. 일반 일정은 계속 사용할 수 있습니다.',
		saveError: '일정을 저장하지 못했습니다.',
		deleteError: '일정을 삭제하지 못했습니다.',
		calendarEventVersionConflictError:
			'다른 곳에서 이 일정이 변경되었습니다. 서버의 최신 내용을 다시 불러왔습니다.',
		calendarDeleteVersionConflictError:
			'다른 곳에서 이 일정이 변경되어 삭제하지 못했습니다. 서버의 최신 내용을 다시 불러왔습니다.',
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
		filterParticipant: '참여자 선택',
		allParticipants: '전체',
		noDayEvents: '일정 없음',
		monthMoreAriaLabel: '{date}의 숨겨진 일정 {count}개 보기',
		new: '일정 추가',
		newEvent: '새 일정',
		editEvent: '일정 편집',
		deleteEvent: '일정 삭제',
		duplicateEvent: '일정 복제',
		addEventOnDay: '이 날짜에 일정 추가',
		openDay: '이 날짜 열기',
		draftPopover: {
			calendar: '캘린더',
			participants: '참여자',
			participantsPlaceholder: '이름 검색',
			participantsEmpty: '검색 결과 없음',
			participantsSummary: '{name} 외 {count}명',
			cancel: '취소',
			complete: '완료',
			delete: '삭제',
			startDate: '시작 날짜',
			endDate: '종료 날짜',
			startTime: '시작 시간',
			endTime: '종료 시간',
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
		holidays: 'Public holidays',
		previousMonth: 'Previous month',
		nextMonth: 'Next month',
		previousWeek: 'Previous week',
		nextWeek: 'Next week',
		previousDay: 'Previous day',
		nextDay: 'Next day',
		pickMonthAndYear: 'Pick month and year',
		previousYear: 'Previous year',
		nextYear: 'Next year',
		previousTwelveYears: 'Previous 12 years',
		nextTwelveYears: 'Next 12 years',
		subscriptionSettings: 'Subscription settings',
		subscriptionReady: 'Subscription URL ready',
		subscriptionShownOnce: 'A subscription URL is shown once, when it is made. Make another if it was lost.',
		shared: 'Shared',
		settings: 'Settings',
		refresh: 'Refresh',
		syncTitle: 'Settings',
		syncDescription: 'Review the URL a calendar app subscribes to.',
		ics: 'ICS',
		rotate: 'Rotate subscription URL',
		copy: 'Copy',
		loading: 'Loading calendar...',
		empty: 'Drag or click the calendar to create an event.',
		error: 'Could not load the calendar.',
		holidayLoadError: 'Could not load public holidays. Other calendar events remain available.',
		saveError: 'Could not save the event.',
		deleteError: 'Could not delete the event.',
		calendarEventVersionConflictError:
			'This event changed elsewhere. The latest server version has been reloaded.',
		calendarDeleteVersionConflictError:
			'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.',
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
		filterParticipant: 'Select participant',
		allParticipants: 'All',
		noDayEvents: 'No events',
		monthMoreAriaLabel: 'Show {count} more events on {date}',
		new: 'Add event',
		newEvent: 'New Event',
		editEvent: 'Edit Event',
		deleteEvent: 'Delete event',
		duplicateEvent: 'Duplicate event',
		addEventOnDay: 'Add event on this day',
		openDay: 'Open this day',
		draftPopover: {
			calendar: 'Calendar',
			participants: 'Participants',
			participantsPlaceholder: 'Search by name',
			participantsEmpty: 'No matches',
			participantsSummary: '{name} +{count}',
			cancel: 'Cancel',
			complete: 'Done',
			delete: 'Delete',
			startDate: 'Start date',
			endDate: 'End date',
			startTime: 'Start time',
			endTime: 'End time',
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
