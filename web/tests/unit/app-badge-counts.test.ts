import { describe, expect, test } from 'bun:test';
import { participatingEventCount, requestedTaskCount } from '../../src/lib/components/app-badge-counts';
import type { CalendarEvent } from '../../src/routes/calendar/embed/calendar-event-persistence';
import type { FlowState, FlowTask } from '../../src/routes/flow/flow-types';

function calendarEvent(id: string, participantEmails: string[], endISO = '2026-07-30T02:00:00+09:00'): CalendarEvent {
	return {
		id,
		uid: id,
		title: id,
		description: '',
		location: '',
		startISO: '2026-07-30T01:00:00+09:00',
		endISO,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: participantEmails.map((email) => ({ personID: email, name: email, email })),
		createdByEmail: 'admin@example.com',
		createdByName: 'admin',
		updatedAt: '2026-07-30T01:00:00+09:00'
	};
}

function flowTask(id: string, status: string, ownerID: string, participantIDs: string[]): FlowTask {
	return {
		id,
		ownerID,
		ownerName: ownerID,
		participantIDs,
		participantNames: participantIDs,
		business: '',
		type: '',
		content: id,
		goal: '',
		size: '',
		status,
		statusRank: 0,
		weekCode: '2026-W31',
		flag: 0
	};
}

function flowState(tasks: FlowTask[]): FlowState {
	return {
		members: [
			{
				id: 'member-1',
				name: '이영희',
				email: 'lee@example.com',
				role: 'member',
				mattermostStatus: 'online',
				activeTaskCount: 0,
				completeTaskCount: 0
			},
			{
				id: 'member-2',
				name: '김철수',
				email: 'kim@example.com',
				role: 'member',
				mattermostStatus: 'online',
				activeTaskCount: 0,
				completeTaskCount: 0
			}
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
		currentUserEmail: 'lee@example.com',
		currentUserName: '이영희',
		isAdmin: false,
		source: 'test'
	};
}

describe('sidebar badge counts', () => {
	const now = new Date('2026-07-30T01:30:00+09:00');

	test('counts only the unfinished events the viewer participates in', () => {
		const events = [
			calendarEvent('mine', ['lee@example.com', 'kim@example.com']),
			calendarEvent('theirs', ['kim@example.com']),
			calendarEvent('mine-upper', ['LEE@example.com']),
			calendarEvent('mine-finished', ['lee@example.com'], '2026-07-30T01:00:00+09:00')
		];
		expect(participatingEventCount(events, 'lee@example.com', now)).toBe(2);
	});

	test('counts nothing without a viewer email', () => {
		expect(participatingEventCount([calendarEvent('mine', ['lee@example.com'])], '  ', now)).toBe(0);
	});

	test('counts requested tasks that name the viewer', () => {
		const tasks = [
			flowTask('owned-request', '요청', 'member-1', []),
			flowTask('participating-request', '요청', 'member-2', ['member-1']),
			flowTask('other-request', '요청', 'member-2', []),
			flowTask('owned-progress', '진행', 'member-1', [])
		];
		expect(requestedTaskCount(flowState(tasks))).toBe(2);
	});
});
