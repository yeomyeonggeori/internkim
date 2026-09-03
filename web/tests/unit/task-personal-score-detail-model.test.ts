// Flow 개인 상세 점수 계산 행을 검증한다.
import { describe, expect, test } from 'bun:test';
import { buildTaskPersonalScoreDetail } from '../../src/routes/task/task-personal-score-detail-model';
import type { TaskDefinitions, TaskMember, TaskSummary, Task } from '../../src/routes/task/task-types';

describe('flow personal score detail model', () => {
	test('builds weekly and monthly score rows for the signed-in member', () => {
		const summary = taskSummary({
			currentUserEmail: 'member@example.com',
			members: [taskMember({ id: 'member-1', email: 'member@example.com' })],
			tasks: [
				task({ id: 'current-week', endDate: '2026-06-03', size: 'D4' }),
				task({ id: 'previous-week', endDate: '2026-05-27', size: 'D2' }),
				task({ id: 'other-member', participantIDs: ['member-2'], participantNames: ['다른 사람'], endDate: '2026-06-04', size: 'D4' }),
				task({ id: 'not-completed', status: 'in_progress', endDate: '2026-06-05', size: 'D4' })
			]
		});

		const detail = buildTaskPersonalScoreDetail(summary);

		expect(detail?.memberName).toBe('김철수');
		expect(detail?.weekly.rows.map((row) => row.completedDistance)).toEqual([4, 2, 0, 0, 0]);
		expect(detail?.weekly.rows[0]).toMatchObject({
			completedDistance: 4,
			cumulativeAverage: 1.2,
			unitScore: 3.333,
			weight: 1.5,
			weightedScore: 76.923
		});
		expect(detail?.weekly.totalScore).toBe(113);
		expect(detail?.monthly.rows.map((row) => row.completedDistance)).toEqual([4, 2, 0, 0, 0]);
		expect(detail?.monthly.totalScore).toBe(113);
	});

	test('returns null when the current user is not a Flow member', () => {
		expect(buildTaskPersonalScoreDetail(taskSummary({ currentUserEmail: 'missing@example.com' }))).toBe(null);
	});
});

function taskSummary(overrides: Partial<TaskSummary>): TaskSummary {
	return {
		week: {
			code: '26W23',
			startISO: '2026-06-01',
			endISO: '2026-06-07',
			previous: '26W22',
			next: '26W24',
			isCurrent: true
		},
		members: [taskMember({})],
		tasks: [],
		metrics: {
			totalTasks: 0,
			completedTasks: 0,
			requestedTasks: 0,
			pausedTasks: 0,
			stoppedTasks: 0,
			statusCounts: {},
			businessCounts: {},
			typeCounts: {}
		},
		definitions: taskDefinitions(),
		statusOptions: [],
		currentUserEmail: 'member@example.com',
		currentUserName: '김철수',
		isAdmin: false,
		...overrides
	};
}

function taskDefinitions(): TaskDefinitions {
	return {
		categories: [],
		types: ['기능'],
		sizes: [
			{ name: 'D2', distanceKm: 2, maxHours: 2, developmentExample: '', otherExample: '', note: '', score: 2, label: 'D2' },
			{ name: 'D4', distanceKm: 4, maxHours: 4, developmentExample: '', otherExample: '', note: '', score: 4, label: 'D4' }
		]
	};
}

function taskMember(overrides: Partial<TaskMember>): TaskMember {
	return {
		id: 'member-1',
		name: '김철수',
		email: 'member@example.com',
		role: 'member',
		mattermostStatus: '',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...overrides
	};
}

function task(overrides: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '김철수',
		participantIDs: ['member-1'],
		participantNames: ['김철수'],
		business: '샘플거리',
		type: '기능',
		content: '업무',
		size: 'D2',
		status: 'completed',
		statusRank: 0,
		startDate: '2026-06-01',
		endDate: '2026-06-03',
		weekCode: '26W23',
		...overrides
	};
}
