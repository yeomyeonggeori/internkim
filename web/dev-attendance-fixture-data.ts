import type { AttendancePresence, AttendanceSummary } from './src/routes/attendance/attendance-context.svelte';

export type DevAttendancePerson = {
	email: string;
	name: string;
	mattermostUsername: string;
	baseHour: number;
	baseMinute: number;
};

export type DevAttendanceLocation = AttendanceSummary['locations'][number];

export const devAttendancePeople: DevAttendancePerson[] = [
	{ email: 'kim@example.com', name: '김철수', mattermostUsername: 'kim', baseHour: 8, baseMinute: 50 },
	{ email: 'member1@example.com', name: '이영희', mattermostUsername: 'lee', baseHour: 8, baseMinute: 45 },
	{ email: 'park@example.com', name: '박지민', mattermostUsername: 'park', baseHour: 9, baseMinute: 5 },
	{ email: 'choi@example.com', name: '최민준', mattermostUsername: 'choi', baseHour: 9, baseMinute: 10 },
	{ email: 'jung@example.com', name: '정수아', mattermostUsername: 'jung', baseHour: 8, baseMinute: 55 },
	{ email: 'kang@example.com', name: '강민호', mattermostUsername: 'kang', baseHour: 9, baseMinute: 20 },
];

export const devAttendanceLocations: AttendanceSummary['locations'] = [
	{ id: 'office', name: '사무실', color: '#22c55e', isDefault: true },
	{ id: 'remote', name: '재택', color: '#3b82f6', isDefault: false },
	{ id: 'bss', name: 'BSS', color: '#0ea5e9', isDefault: false },
	{ id: 'outside', name: '외부', color: '#f59e0b', isDefault: false },
];

export const devAttendancePresences: Record<string, AttendancePresence> = {
	'kim@example.com': 'online',
	'member1@example.com': 'away',
	'park@example.com': 'online',
	'choi@example.com': 'offline',
	'jung@example.com': 'dnd',
	'kang@example.com': 'offline',
};
