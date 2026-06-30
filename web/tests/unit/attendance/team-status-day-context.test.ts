import { describe, expect, test } from 'bun:test';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { FlowState, FlowTask } from '../../../src/routes/flow/flow-types';
import { buildTeamStatusDayContext, type TeamStatusDayContextLoadState } from '../../../src/routes/attendance/team/team-status-day-context';

describe('team status day context', () => {
	test('shows only personal calendar events and hides all-participant events', () => {
		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수', mattermostUsername: 'kim' },
			'2026-06-16',
			[
				calendarEvent('personal-email', '개인 일정', '2026-06-16T01:00:00.000Z', '2026-06-16T02:00:00.000Z', [
					{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
				]),
				calendarEvent('all-id', '전체 일정', '2026-06-16T03:00:00.000Z', '2026-06-16T04:00:00.000Z', [
					{ personID: 'all', name: '전체' }
				]),
				calendarEvent('other', '다른 사람 일정', '2026-06-16T05:00:00.000Z', '2026-06-16T06:00:00.000Z', [
					{ personID: 'park', name: '박지민', email: 'park@example.com' }
				])
			],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.calendarEvents.map((event) => event.title)).toEqual(['개인 일정']);
	});

	test('matches legacy people and all-day date ranges', () => {
		const legacyEvent = {
			...calendarEvent('legacy', '레거시 일정', '2026-06-15T00:00:00.000Z', '2026-06-17T00:00:00.000Z', []),
			isAllDay: true,
			people: ['김철수']
		} satisfies CalendarEvent & { people: string[] };

		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-16',
			[legacyEvent],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.calendarEvents).toEqual([
			{
				id: 'legacy',
				title: '레거시 일정',
				timeLabel: '종일'
			}
		]);
	});

	test('uses exact calendar participant tokens before name fallback', () => {
		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수', mattermostUsername: 'kim' },
			'2026-06-16',
			[
				calendarEvent('same-name-other-email', '동명이인 일정', '2026-06-16T01:00:00.000Z', '2026-06-16T02:00:00.000Z', [
					{ personID: 'same-name', name: '김철수', email: 'same-name@example.com' }
				]),
				calendarEvent('email-matched', '이메일 매칭 일정', '2026-06-16T03:00:00.000Z', '2026-06-16T04:00:00.000Z', [
					{ personID: 'other-id', name: '다른 표시명', email: 'kim@example.com' }
				]),
				calendarEvent('id-matched', 'ID 매칭 일정', '2026-06-16T05:00:00.000Z', '2026-06-16T06:00:00.000Z', [
					{ personID: 'kim', name: '다른 표시명', email: 'other@example.com' }
				])
			],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.calendarEvents.map((event) => event.id)).toEqual(['email-matched', 'id-matched']);
	});

	test('hides timed events from the end date when the event ends at midnight', () => {
		const event = calendarEvent('midnight-end', '자정 종료 일정', '2026-06-16T14:00:00.000Z', '2026-06-16T15:00:00.000Z', [
			{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
		]);
		const dayContext = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-16',
			[event],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);
		const nextDayContext = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-17',
			[event],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(dayContext.calendarEvents.map((calendarEvent) => calendarEvent.id)).toEqual(['midnight-end']);
		expect(nextDayContext.calendarEvents).toEqual([]);
	});

	test('shows completed flow tasks for the selected person and date', () => {
		const flowState = flowStateFixture([
			flowTask('done-owned', '완료', '2026-06-16', 'kim', ['kim'], ['김철수']),
			flowTask('done-owned-collaborating', '완료', '2026-06-16', 'kim', ['kim', 'park'], ['김철수', '박지민']),
			flowTask('done-participating', '완료', '2026-06-16', 'park', ['park', 'kim'], ['박지민', '김철수']),
			flowTask('planned', '예정', '2026-06-16', 'kim', ['kim'], ['김철수']),
			flowTask('other-date', '완료', '2026-06-17', 'kim', ['kim'], ['김철수'])
		]);

		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-16',
			[],
			flowState,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.completedTasks.map((task) => task.id)).toEqual(['done-owned', 'done-owned-collaborating', 'done-participating']);
		expect(context.completedTasks.find((task) => task.id === 'done-owned')?.collaboratorNames).toEqual([]);
		expect(context.completedTasks.find((task) => task.id === 'done-owned-collaborating')?.collaboratorNames).toEqual(['박지민']);
		expect(context.completedTasks.find((task) => task.id === 'done-participating')?.collaboratorNames).toEqual(['김철수']);
	});

	test('uses email matched flow member IDs before name fallback', () => {
		const flowState = flowStateFixture([
			flowTask('same-name-other-email', '완료', '2026-06-16', 'same-name', ['same-name'], ['김철수']),
			flowTask('email-matched', '완료', '2026-06-16', 'kim', ['kim'], ['김철수'])
		]);
		flowState.members.push(flowMember('same-name', '김철수', 'same-name@example.com'));

		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-16',
			[],
			flowState,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.completedTasks.map((task) => task.id)).toEqual(['email-matched']);
	});

	test('hides calendar events with invalid dates', () => {
		const context = buildTeamStatusDayContext(
			{ email: 'kim@example.com', displayName: '김철수' },
			'2026-06-16',
			[
				calendarEvent('invalid-start', '시작일 오류', 'not-a-date', '2026-06-16T02:00:00.000Z', [
					{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
				]),
				calendarEvent('invalid-end', '종료일 오류', '2026-06-16T01:00:00.000Z', 'not-a-date', [
					{ personID: 'kim', name: '김철수', email: 'kim@example.com' }
				])
			],
			null,
			'ko-KR',
			'종일',
			loadedContext()
		);

		expect(context.calendarEvents).toEqual([]);
	});
});

function loadedContext(): TeamStatusDayContextLoadState {
	return {
		isCalendarEventsLoading: false,
		isCompletedWorkLoading: false,
		hasCalendarEventsLoadFailed: false,
		hasCompletedWorkLoadFailed: false
	};
}

function calendarEvent(
	id: string,
	title: string,
	startISO: string,
	endISO: string,
	participants: CalendarEvent['participants']
): CalendarEvent {
	return {
		id,
		uid: id,
		title,
		description: '',
		location: '',
		startISO,
		endISO,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants,
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: startISO
	};
}

function flowStateFixture(tasks: FlowTask[]): FlowState {
	return {
		members: [
			flowMember('kim', '김철수', 'kim@example.com'),
			flowMember('park', '박지민', 'park@example.com')
		],
		tasks,
		metrics: {
			totalTasks: tasks.length,
			completedTasks: 0,
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

function flowTask(
	id: string,
	status: string,
	endDate: string,
	ownerID: string,
	participantIDs: string[],
	participantNames: string[]
): FlowTask {
	return {
		id,
		ownerID,
		ownerName: participantNames[participantIDs.indexOf(ownerID)] ?? ownerID,
		participantIDs,
		participantNames,
		business: '',
		type: '',
		content: `${id} 업무`,
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
