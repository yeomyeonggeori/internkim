// 근태 팀 현황 E2E에서 쓰는 캘린더와 Flow 컨텍스트 fixture를 만든다.
import type { Page } from '@playwright/test';
import type { CalendarEvent } from '../../src/routes/calendar/embed/calendar-event-persistence';
import type { FlowState, FlowTask } from '../../src/routes/flow/flow-types';

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
