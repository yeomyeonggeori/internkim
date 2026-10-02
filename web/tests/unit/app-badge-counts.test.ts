import { describe, expect, test } from 'bun:test';
import { participatingEventCount, requestedTaskCount } from '../../src/lib/components/app-badge-counts';
import type { CalendarEvent } from '../../src/routes/calendar/embed/calendar-event-persistence';
import type { TaskState, Task } from '../../src/routes/task/task-types';

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

function task(id: string, status: string, ownerID: string, participantIDs: string[]): Task {
	return {
		id,
		ownerID,
		ownerName: ownerID,
		participantIDs,
		participantNames: participantIDs,
		business: '',
		type: '',
		content: id,
		size: '',
		status,
		weekCode: '2026-W31'
	};
}

function taskState(tasks: Task[]): TaskState {
	return {
		completeness: 'full',
		peopleReady: true,
		members: [
			{
				id: 'member-1',
				name: '이영희',
				email: 'member1@example.com',
				role: 'member',
				activeTaskCount: 0,
				completeTaskCount: 0
			},
			{
				id: 'member-2',
				name: '김철수',
				email: 'kim@example.com',
				role: 'member',
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
		currentUserEmail: 'member1@example.com',
		currentUserName: '이영희',
		isAdmin: false,
	};
}

describe('sidebar badge counts', () => {
	const now = new Date('2026-07-30T01:30:00+09:00');

	test('counts only the unfinished events the viewer participates in', () => {
		const events = [
			calendarEvent('mine', ['member1@example.com', 'kim@example.com']),
			calendarEvent('theirs', ['kim@example.com']),
			calendarEvent('mine-upper', ['MEMBER1@example.com']),
			calendarEvent('mine-finished', ['member1@example.com'], '2026-07-30T01:00:00+09:00')
		];
		expect(participatingEventCount(events, 'member1@example.com', now)).toBe(2);
	});

	test('counts nothing without a viewer email', () => {
		expect(participatingEventCount([calendarEvent('mine', ['member1@example.com'])], '  ', now)).toBe(0);
	});

	test('counts requested tasks that name the viewer', () => {
		const tasks = [
			task('owned-request', 'requested', 'member-1', []),
			task('participating-request', 'requested', 'member-2', ['member-1']),
			task('other-request', 'requested', 'member-2', []),
			task('owned-progress', 'in_progress', 'member-1', [])
		];
		expect(requestedTaskCount(taskState(tasks))).toBe(2);
	});
});
