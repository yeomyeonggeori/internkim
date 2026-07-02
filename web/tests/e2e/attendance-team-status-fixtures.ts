import type { Page } from '@playwright/test';
import { buildAttendanceSummaryFixture } from '../../dev-attendance-summary-fixture';
import type {
	AttendanceEvent,
	AttendanceSummary
} from '../../src/routes/attendance/attendance-context.svelte';
import type { CalendarEvent } from '../../src/routes/calendar/embed/calendar-event-persistence';
import type { FlowState, FlowTask } from '../../src/routes/flow/flow-types';

export const overflowTargetDate = '2026-06-16';

export async function routePersonalDayContext(page: Page, targetDate: string): Promise<void> {
	await page.unroute('**/calendar/api/events?**');
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					calendarEvent('personal-calendar-event', '개인 캘린더 일정', targetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					])
				]
			}
		});
	});
	await page.unroute('**/flow/api/state');
	await page.route('**/flow/api/state', async (route) => {
		await route.fulfill({
			json: flowStateFixture([
				flowTask('completed-personal-task', '월간 현황 팝업 구현', '완료', targetDate, 'kim', ['kim'])
			])
		});
	});
}

export async function routeSourceSeparatedDayContext(page: Page, targetDate: string): Promise<void> {
	await page.unroute('**/calendar/api/events?**');
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					calendarEvent('calendar-shaped-as-work', '완료 업무처럼 보이는 캘린더', targetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					]),
					calendarEvent('calendar-other-person-work-title', '다른 사람 완료 업무처럼 보이는 캘린더', targetDate, [
						{ personID: 'park', name: '박지민', email: 'park@example.com' }
					])
				]
			}
		});
	});
	await page.unroute('**/flow/api/state');
	await page.route('**/flow/api/state', async (route) => {
		await route.fulfill({
			json: flowStateFixture([
				flowTask('flow-shaped-as-calendar', '캘린더 일정처럼 보이는 완료 업무', '완료', targetDate, 'kim', ['kim']),
				flowTask('flow-planned-calendar-title', '캘린더 일정처럼 보이는 예정 업무', '예정', targetDate, 'kim', ['kim']),
				flowTask('flow-other-person-calendar-title', '다른 사람 캘린더 일정처럼 보이는 완료 업무', '완료', targetDate, 'park', ['park'])
			])
		});
	});
}

export async function routeOverflowStatusDay(page: Page): Promise<void> {
	await page.unroute('**/attendance/api/summary**');
	await page.route('**/attendance/api/summary**', async (route) => {
		const requestURL = new URL(route.request().url());
		const month = requestURL.searchParams.get('month') ?? '2026-06';
		await route.fulfill({
			json: month === '2026-06' ? buildOverflowStatusSummary() : buildAttendanceSummaryFixture(month)
		});
	});
	await page.unroute('**/calendar/api/events?**');
	await page.route('**/calendar/api/events?**', async (route) => {
		await route.fulfill({
			json: {
				events: [
					calendarEvent('overflow-calendar-1', '오전 스탠드업', overflowTargetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					]),
					calendarEvent('overflow-calendar-2', '협업 일정 리뷰', overflowTargetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					]),
					calendarEvent('overflow-calendar-3', '운영 정책 미팅', overflowTargetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					]),
					calendarEvent('overflow-calendar-4', '퇴근 전 동기화', overflowTargetDate, [
						{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
					])
				]
			}
		});
	});
	await page.unroute('**/flow/api/state');
	await page.route('**/flow/api/state', async (route) => {
		await route.fulfill({
			json: flowStateFixture([
				flowTask('overflow-task-1', '출결 대시보드 점검', '완료', overflowTargetDate, 'kim', ['kim']),
				flowTask('overflow-task-2', '근무 기록 정합성 확인', '완료', overflowTargetDate, 'kim', ['kim']),
				flowTask('overflow-task-3', '월간 현황 QA', '완료', overflowTargetDate, 'kim', ['kim']),
				flowTask('overflow-task-4', '일정 연동 확인', '완료', overflowTargetDate, 'kim', ['kim'])
			])
		});
	});
}

export function calendarEvent(
	id: string,
	title: string,
	date: string,
	participants: NonNullable<CalendarEvent['participants']>
): CalendarEvent {
	return {
		id,
		uid: id,
		title,
		description: '',
		location: '회의실 A',
		startISO: `${date}T10:00:00+09:00`,
		endISO: `${date}T11:00:00+09:00`,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants,
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: `${date}T09:00:00+09:00`
	};
}

function buildOverflowStatusSummary(): AttendanceSummary {
	const summary = buildAttendanceSummaryFixture('2026-06');
	return {
		...summary,
		events: [
			...summary.events.filter((event) => event.email !== 'kim@example.com' || event.localDate !== overflowTargetDate),
			overflowAttendanceEvent('overflow-office-in-1', 'clock_in', '09:00', 'office', '사무실'),
			overflowAttendanceEvent('overflow-office-out-1', 'clock_out', '09:45', 'office', '사무실'),
			overflowAttendanceEvent('overflow-lab-in', 'clock_in', '10:00', 'lab', '실험실'),
			overflowAttendanceEvent('overflow-lab-out', 'clock_out', '10:45', 'lab', '실험실'),
			overflowAttendanceEvent('overflow-meeting-in', 'clock_in', '11:00', 'meeting', '회의실'),
			overflowAttendanceEvent('overflow-meeting-out', 'clock_out', '11:45', 'meeting', '회의실'),
			overflowAttendanceEvent('overflow-office-in-2', 'clock_in', '12:00', 'office', '사무실'),
			overflowAttendanceEvent('overflow-office-out-2', 'clock_out', '12:45', 'office', '사무실')
		],
		locations: [
			{ id: 'office', name: '사무실', color: '#22c55e', isDefault: true },
			{ id: 'lab', name: '실험실', color: '#2563eb', isDefault: false },
			{ id: 'meeting', name: '회의실', color: '#f59e0b', isDefault: false }
		]
	};
}

function overflowAttendanceEvent(
	id: string,
	kind: AttendanceEvent['kind'],
	localTime: string,
	locationID: string,
	locationName: string
): AttendanceEvent {
	return {
		id,
		mattermostUserID: 'kim',
		mattermostUsername: 'kim',
		email: 'kim@example.com',
		displayName: '김철수',
		kind,
		occurredAt: `${overflowTargetDate}T${localTime}:00+09:00`,
		localDate: overflowTargetDate,
		localTime,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'test',
		resultPostID: `${id}-post`,
		locationID,
		locationName
	};
}

export function flowStateFixture(tasks: FlowTask[]): FlowState {
	return {
		members: [
			flowMember('kim', '김철수', 'kim@example.com'),
			flowMember('park', '박지민', 'park@example.com')
		],
		tasks,
		metrics: {
			totalTasks: tasks.length,
			completedTasks: 1,
			requestedTasks: 0,
			pausedTasks: 0,
			stoppedTasks: 0,
			statusCounts: {},
			businessCounts: {},
			typeCounts: {}
		},
		definitions: { categories: [], types: [], sizes: [] },
		statusOptions: [],
		currentUserEmail: 'kim@example.com',
		currentUserName: '김철수',
		isAdmin: true,
		source: 'test'
	};
}

function flowMember(id: string, name: string, email: string): FlowState['members'][number] {
	return {
		id,
		name,
		email,
		role: 'member',
		mattermostStatus: 'active',
		activeTaskCount: 0,
		completeTaskCount: 0
	};
}

export function flowTask(
	id: string,
	content: string,
	status: string,
	endDate: string,
	ownerID: string,
	participantIDs: string[]
): FlowTask {
	const namesByID: Record<string, string> = { kim: '김철수', park: '박지민' };
	return {
		id,
		ownerID,
		ownerName: namesByID[ownerID] ?? ownerID,
		participantIDs,
		participantNames: participantIDs.map((participantID) => namesByID[participantID] ?? participantID),
		business: '',
		type: '',
		content,
		goal: '',
		size: 'M',
		status,
		statusRank: 0,
		startDate: '2026-06-15',
		endDate,
		weekCode: '2026-W25',
		flag: 0
	};
}
