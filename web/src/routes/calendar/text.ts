export const calendarText = {
	ko: {
		title: 'Calendar',
		subtitle: '팀 일정과 외부 캘린더 연동을 한 화면에서 다룹니다.',
		shared: 'Shared',
		refresh: '새로고침',
		syncTitle: '연동',
		syncDescription: 'Google은 구독 URL로 읽고, Apple 캘린더는 CalDAV로 연결합니다.',
		caldav: 'CalDAV',
		ics: 'Google iCal',
		rotate: '구독 URL 재발급',
		copy: '복사',
		loading: '캘린더를 불러오는 중...',
		empty: '일정을 드래그하거나 더블클릭해서 새 일정을 만드세요.',
		error: '캘린더를 불러오지 못했습니다.',
		saveError: '일정을 저장하지 못했습니다.',
		deleteError: '일정을 삭제하지 못했습니다.',
		today: '오늘'
	},
	en: {
		title: 'Calendar',
		subtitle: 'Team schedule and external calendar sync in one place.',
		shared: 'Shared',
		refresh: 'Refresh',
		syncTitle: 'Sync',
		syncDescription: 'Google reads the subscription URL. Apple Calendar connects through CalDAV.',
		caldav: 'CalDAV',
		ics: 'Google iCal',
		rotate: 'Rotate subscription URL',
		copy: 'Copy',
		loading: 'Loading calendar...',
		empty: 'Drag or double-click the calendar to create an event.',
		error: 'Could not load the calendar.',
		saveError: 'Could not save the event.',
		deleteError: 'Could not delete the event.',
		today: 'Today'
	}
} as const;
