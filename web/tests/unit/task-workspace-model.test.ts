import { describe, expect, test } from 'bun:test';
import {
	ETC_TASK_OPTION_VALUE,
	buildBusinessFilterOptions,
	buildTypeSelectOptions,
	buildTaskTabs,
	buildMemberFilterOptions,
	canDeleteTask,
	canRemoveTaskParticipant,
	canManageTaskAssignment,
	canUpdateTask,
	defaultParticipantFilterIDs,
	filterTasks,
	taskDefinitionLabel,
	taskDefinitionOptionValue,
	taskDefinitionValueFromOption,
	isDefaultParticipantFilter,
	sortTaskList
} from '../../src/routes/task/task-workspace-model';
import type { TaskMember, TaskSummary, Task } from '../../src/routes/task/task-types';

describe('flow task workspace model', () => {
	test('opens the task workspace without personal member tabs', () => {
		expect(buildTaskTabs()).toEqual(['tasks', 'report', 'definitions', 'members']);
	});

	test('defaults participant filtering to the signed-in member', () => {
		const summary = taskSummary({
			currentUserEmail: 'member1@example.com',
			members: [
				taskMember({ id: 'designer', email: 'kim@example.com' }),
				taskMember({ id: 'engineer', email: 'member1@example.com' })
			]
		});

		expect(defaultParticipantFilterIDs(summary)).toEqual(['engineer']);
		expect(isDefaultParticipantFilter(['engineer'], summary)).toBe(true);
		expect(isDefaultParticipantFilter(['engineer', 'designer'], summary)).toBe(false);
	});

	test('filters tasks by multiple participants and null business', () => {
		const tasks = [
			taskOf({ id: 'task-1', participantIDs: ['engineer'], business: null }),
			taskOf({ id: 'task-2', participantIDs: ['designer'], business: '샘플거리' }),
			taskOf({ id: 'task-3', participantIDs: ['engineer', 'designer'], business: '샘플거리' })
		];

		expect(filterTasks(tasks, {
			searchText: '',
			statusFilter: 'all',
			participantFilterIDs: ['engineer'],
			businessFilter: 'all',
			typeFilter: 'all'
		}).map((task) => task.id)).toEqual(['task-1', 'task-3']);
		expect(filterTasks(tasks, {
			searchText: '',
			statusFilter: 'all',
			participantFilterIDs: ['engineer', 'designer'],
			businessFilter: ETC_TASK_OPTION_VALUE,
			typeFilter: 'all'
		}).map((task) => task.id)).toEqual(['task-1']);
	});

	test('filters tasks with a null type through the etc option', () => {
		const tasks = [
			taskOf({ id: 'task-1', type: null }),
			taskOf({ id: 'task-2', type: '기능' })
		];

		expect(filterTasks(tasks, {
			searchText: '',
			statusFilter: 'all',
			participantFilterIDs: [],
			businessFilter: 'all',
			typeFilter: ETC_TASK_OPTION_VALUE
		}).map((task) => task.id)).toEqual(['task-1']);
	});

	test('labels a null value with the localized etc label', () => {
		expect(taskDefinitionLabel(null, 'Etc.')).toBe('Etc.');
		expect(taskDefinitionLabel('샘플거리', 'Etc.')).toBe('샘플거리');
		expect(buildBusinessFilterOptions(['샘플거리'], '전체', '기타')).toEqual([
			{ value: 'all', label: '전체' },
			{ value: ETC_TASK_OPTION_VALUE, label: '기타' },
			{ value: '샘플거리', label: '샘플거리' }
		]);
		expect(buildTypeSelectOptions({ categories: [], types: ['기능'], sizes: [] }, 'Etc.')).toEqual([
			{ value: ETC_TASK_OPTION_VALUE, label: 'Etc.' },
			{ value: '기능', label: '기능' }
		]);
	});

	test('maps the etc select option to a null persisted value', () => {
		expect(taskDefinitionOptionValue(null)).toBe(ETC_TASK_OPTION_VALUE);
		expect(taskDefinitionOptionValue('기능')).toBe('기능');
		expect(taskDefinitionValueFromOption(ETC_TASK_OPTION_VALUE)).toBe(null);
		expect(taskDefinitionValueFromOption('기능')).toBe('기능');
	});

	test('keeps member profile images in participant filter options', () => {
		expect(buildMemberFilterOptions([
			taskMember({
				id: 'owner',
				name: '담당자',
				email: 'owner@example.com',
				image: '/calendar/api/participants/owner/image'
			})
		], '전체')).toEqual([
			{ value: 'all', label: '전체' },
			{
				value: 'owner',
				label: '담당자',
				email: 'owner@example.com',
				image: '/calendar/api/participants/owner/image'
			}
		]);
	});

	test('grants the sole participant task and assignment authority', () => {
		const summary = taskSummary({
			currentUserEmail: 'member1@example.com',
			isAdmin: false,
			members: [
				taskMember({ id: 'participant', email: 'member1@example.com' }),
				taskMember({ id: 'viewer', email: 'viewer@example.com' })
			]
		});
		const task = taskOf({ ownerID: 'participant', participantIDs: ['participant'] });

		expect(canUpdateTask(summary, task)).toBe(true);
		expect(canDeleteTask(summary, task)).toBe(true);
		expect(canManageTaskAssignment(summary, task)).toBe(true);
		expect(canRemoveTaskParticipant(task, 'participant')).toBe(false);
	});

	test('lets every participant update shared tasks without order-based authority', () => {
		const summary = taskSummary({
			currentUserEmail: 'member1@example.com',
			isAdmin: false,
			members: [
				taskMember({ id: 'owner', email: 'owner@example.com' }),
				taskMember({ id: 'participant', email: 'member1@example.com' })
			]
		});
		const task = taskOf({
			ownerID: '',
			participantIDs: ['owner', 'participant']
		});

		expect(canUpdateTask(summary, task)).toBe(true);
		expect(canManageTaskAssignment(summary, task)).toBe(false);
		expect(canDeleteTask(summary, task)).toBe(false);
		expect(canUpdateTask({ ...summary, currentUserEmail: 'owner@example.com' }, task)).toBe(true);
		expect(canManageTaskAssignment({ ...summary, currentUserEmail: 'owner@example.com' }, task)).toBe(false);
		expect(canDeleteTask({ ...summary, currentUserEmail: 'owner@example.com' }, task)).toBe(false);
		expect(canManageTaskAssignment({ ...summary, isAdmin: true }, task)).toBe(true);
		expect(canDeleteTask({ ...summary, isAdmin: true }, task)).toBe(true);
	});

	test('does not grant task authority without participation', () => {
		const summary = taskSummary({
			currentUserEmail: 'member1@example.com',
			isAdmin: false,
			members: [taskMember({ id: 'viewer', email: 'member1@example.com' })]
		});
		const task = taskOf({ ownerID: '', participantIDs: [], participantNames: [] });

		expect(canUpdateTask(summary, task)).toBe(false);
		expect(canDeleteTask(summary, task)).toBe(false);
		expect(canManageTaskAssignment(summary, task)).toBe(false);
		expect(canRemoveTaskParticipant(task, 'viewer')).toBe(false);
	});

	test('lets a requester edit their unsaved request draft without becoming a participant', () => {
		const summary = taskSummary({
			currentUserEmail: 'requester@example.com',
			members: [taskMember({ id: 'requester', email: 'requester@example.com' })]
		});
		const draft = taskOf({
			id: '',
			ownerID: 'target',
			requesterID: 'requester',
			participantIDs: ['target']
		});

		expect(canUpdateTask(summary, draft)).toBe(true);
	});

	test('allows participant removal only when another participant remains', () => {
		const soleTask = taskOf({ ownerID: 'owner', participantIDs: ['owner'] });
		const sharedTask = taskOf({ ownerID: '', participantIDs: ['owner', 'participant'] });

		expect(canRemoveTaskParticipant(soleTask, 'owner')).toBe(false);
		expect(canRemoveTaskParticipant(sharedTask, 'owner')).toBe(true);
		expect(canRemoveTaskParticipant(sharedTask, 'participant')).toBe(true);
	});

	test('sorts the task list by status group, end date, and created date', () => {
		const tasks = [
			taskOf({ id: 'done-new', status: 'completed', endDate: '2026-06-12', createdAt: '2026-06-01T10:00:00Z' }),
			taskOf({ id: 'requested-old', status: 'requested', endDate: '2026-06-01', createdAt: '2026-06-03T10:00:00Z' }),
			taskOf({ id: 'stopped', status: 'stopped', endDate: '2026-06-15', createdAt: '2026-06-04T10:00:00Z' }),
			taskOf({ id: 'requested-new', status: 'requested', endDate: '2026-06-05', createdAt: '2026-06-02T10:00:00Z' }),
			taskOf({ id: 'scheduled-created-new', status: 'planned', endDate: '', createdAt: '2026-06-09T10:00:00Z' }),
			taskOf({ id: 'scheduled-created-old', status: 'planned', endDate: '', createdAt: '2026-06-08T10:00:00Z' })
		];

		expect(sortTaskList(tasks).map((task) => task.id)).toEqual([
			'requested-new',
			'requested-old',
			'scheduled-created-new',
			'scheduled-created-old',
			'stopped',
			'done-new'
		]);
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
		members: [],
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
		definitions: {
			categories: [],
			types: [],
			sizes: []
		},
		statusOptions: [],
		currentUserEmail: '',
		currentUserName: '',
		isAdmin: false,
		...overrides
	};
}

function taskMember(overrides: Partial<TaskMember>): TaskMember {
	return {
		id: 'member-1',
		name: '최견본',
		email: 'member1@example.com',
		role: 'member',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...overrides
	};
}

function taskOf(overrides: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '',
		type: '기능',
		content: '업무',
		size: 'M',
		status: 'planned',
		weekCode: '26W23',
		...overrides
	};
}
