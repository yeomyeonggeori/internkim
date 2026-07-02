export type DevPopupOverflowAttendanceRow = {
	kind: 'clock_in' | 'clock_out';
	localTime: string;
	locationID: string;
	sourceMessage: string;
};

export type DevPopupOverflowCalendarEvent = {
	id: string;
	title: string;
	startTime: string;
	endTime: string;
	location: string;
};

export const devPopupOverflowDate = '2026-06-16';
export const devPopupOverflowMonth = devPopupOverflowDate.slice(0, 7);
export const devPopupOverflowDay = Number(devPopupOverflowDate.slice(8, 10));
export const devPopupOverflowEmail = 'kim@example.com';
export const devPopupOverflowDisplayName = '김철수';

export const devPopupOverflowAttendanceRows: DevPopupOverflowAttendanceRow[] = [
	{ kind: 'clock_in', localTime: '09:00', locationID: 'office', sourceMessage: '오전 사무실 근무 시작' },
	{ kind: 'clock_out', localTime: '09:45', locationID: 'office', sourceMessage: '사무실 구간 종료' },
	{ kind: 'clock_in', localTime: '10:00', locationID: 'bss', sourceMessage: 'BSS 점검 시작' },
	{ kind: 'clock_out', localTime: '10:45', locationID: 'bss', sourceMessage: 'BSS 점검 종료' },
	{ kind: 'clock_in', localTime: '11:00', locationID: 'outside', sourceMessage: '외부 일정 시작' },
	{ kind: 'clock_out', localTime: '11:45', locationID: 'outside', sourceMessage: '외부 일정 종료' },
	{ kind: 'clock_in', localTime: '12:00', locationID: 'office', sourceMessage: '사무실 복귀' },
	{ kind: 'clock_out', localTime: '12:45', locationID: 'office', sourceMessage: '오전 업무 종료' }
];

export const devPopupOverflowCalendarEvents: DevPopupOverflowCalendarEvent[] = [
	{ id: 'dev-calendar-overflow-1', title: '오전 스탠드업', startTime: '09:30', endTime: '10:00', location: '회의실 A' },
	{ id: 'dev-calendar-overflow-2', title: '협업 일정 리뷰', startTime: '10:30', endTime: '11:00', location: '회의실 A' },
	{ id: 'dev-calendar-overflow-3', title: '운영 정책 미팅', startTime: '13:00', endTime: '14:00', location: '회의실 A' },
	{ id: 'dev-calendar-overflow-4', title: '퇴근 전 동기화', startTime: '16:00', endTime: '16:30', location: '회의실 A' }
];

export const devPopupOverflowCompletedTaskTitles = [
	'출결 대시보드 점검',
	'근무 기록 정합성 확인',
	'월간 현황 QA',
	'일정 연동 확인'
];
