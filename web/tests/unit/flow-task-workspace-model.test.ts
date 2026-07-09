// Flow 업무 작업공간 모델의 필터와 권한 규칙을 검증한다.
import { describe, expect, test } from 'bun:test';
import {
	EMPTY_FLOW_BUSINESS_VALUE,
	buildBusinessFilterOptions,
	buildFlowTaskTabs,
	buildMemberFilterOptions,
	canDeleteFlowTask,
	canRemoveFlowTaskParticipant,
	canManageFlowTaskAssignment,
	canUpdateFlowTask,
	defaultParticipantFilterIDs,
	filterFlowTasks,
	flowBusinessLabel,
	isDefaultParticipantFilter,
	sortFlowTaskList
} from '../../src/routes/flow/flow-task-workspace-model';
import type { FlowMember, FlowSummary, FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task workspace model', () => {
	test('opens the task workspace without personal member tabs', () => {
		expect(buildFlowTaskTabs()).toEqual(['tasks', 'report', 'definitions', 'members']);
	});

	test('defaults participant filtering to the signed-in member', () => {
		const summary = flowSummary({
			currentUserEmail: 'lee@example.com',
			members: [
				flowMember({ id: 'designer', email: 'kim@example.com' }),
				flowMember({ id: 'engineer', email: 'lee@example.com' })
			]
		});

		expect(defaultParticipantFilterIDs(summary)).toEqual(['engineer']);
		expect(isDefaultParticipantFilter(['engineer'], summary)).toBe(true);
		expect(isDefaultParticipantFilter(['engineer', 'designer'], summary)).toBe(false);
	});

	test('filters tasks by multiple participants and empty business', () => {
		const tasks = [
			flowTask({ id: 'task-1', participantIDs: ['engineer'], business: '' }),
			flowTask({ id: 'task-2', participantIDs: ['designer'], business: '여명거리' }),
			flowTask({ id: 'task-3', participantIDs: ['engineer', 'designer'], business: '여명거리' })
		];

		expect(filterFlowTasks(tasks, {
			searchText: '',
			statusFilter: 'all',
			participantFilterIDs: ['engineer'],
			businessFilter: 'all',
			typeFilter: 'all'
		}).map((task) => task.id)).toEqual(['task-1', 'task-3']);
		expect(filterFlowTasks(tasks, {
			searchText: '',
			statusFilter: 'all',
			participantFilterIDs: ['engineer', 'designer'],
			businessFilter: EMPTY_FLOW_BUSINESS_VALUE,
			typeFilter: 'all'
		}).map((task) => task.id)).toEqual(['task-1']);
	});

	test('labels empty business as 기타 without changing the persisted value', () => {
		expect(flowBusinessLabel('', '기타')).toBe('기타');
		expect(flowBusinessLabel('여명거리', '기타')).toBe('여명거리');
		expect(buildBusinessFilterOptions(['여명거리'], '전체', '기타')).toEqual([
			{ value: 'all', label: '전체' },
			{ value: EMPTY_FLOW_BUSINESS_VALUE, label: '기타' },
			{ value: '여명거리', label: '여명거리' }
		]);
	});

	test('keeps member profile images in participant filter options', () => {
		expect(buildMemberFilterOptions([
			flowMember({
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

	test('matches server task update and delete permission rules', () => {
		const summary = flowSummary({
			currentUserEmail: 'lee@example.com',
			isAdmin: false,
			members: [
				flowMember({ id: 'owner', email: 'owner@example.com' }),
				flowMember({ id: 'participant', email: 'lee@example.com' }),
				flowMember({ id: 'viewer', email: 'viewer@example.com' })
			]
		});
		const participantTask = flowTask({
			ownerID: 'owner',
			participantIDs: ['participant']
		});
		const viewerTask = flowTask({
			ownerID: 'owner',
			participantIDs: ['viewer']
		});

		expect(canUpdateFlowTask(summary, participantTask)).toBe(true);
		expect(canDeleteFlowTask(summary, participantTask)).toBe(false);
		expect(canUpdateFlowTask(summary, viewerTask)).toBe(false);
		expect(canDeleteFlowTask({ ...summary, isAdmin: true }, viewerTask)).toBe(true);
	});

	test('allows only admins or owners to change task assignment fields', () => {
		const summary = flowSummary({
			currentUserEmail: 'lee@example.com',
			isAdmin: false,
			members: [
				flowMember({ id: 'owner', email: 'owner@example.com' }),
				flowMember({ id: 'participant', email: 'lee@example.com' })
			]
		});
		const task = flowTask({
			ownerID: 'owner',
			participantIDs: ['owner', 'participant']
		});

		expect(canUpdateFlowTask(summary, task)).toBe(true);
		expect(canManageFlowTaskAssignment(summary, task)).toBe(false);
		expect(canManageFlowTaskAssignment({ ...summary, currentUserEmail: 'owner@example.com' }, task)).toBe(true);
		expect(canManageFlowTaskAssignment({ ...summary, isAdmin: true }, task)).toBe(true);
	});

	test('keeps the owner locked in the participant list', () => {
		const task = flowTask({
			ownerID: 'owner',
			participantIDs: ['owner', 'participant'],
			participantNames: ['담당자', '참여자']
		});

		expect(canRemoveFlowTaskParticipant(task, 'owner')).toBe(false);
		expect(canRemoveFlowTaskParticipant(task, 'participant')).toBe(true);
		expect(canRemoveFlowTaskParticipant(task, 'viewer')).toBe(false);
	});

	test('sorts the task list by status group, end date, and created date', () => {
		const tasks = [
			flowTask({ id: 'done-new', status: '완료', endDate: '2026-06-12', createdAt: '2026-06-01T10:00:00Z' }),
			flowTask({ id: 'requested-old', status: '요청', endDate: '2026-06-01', createdAt: '2026-06-03T10:00:00Z' }),
			flowTask({ id: 'stopped', status: '중단', endDate: '2026-06-15', createdAt: '2026-06-04T10:00:00Z' }),
			flowTask({ id: 'requested-new', status: '요청', endDate: '2026-06-05', createdAt: '2026-06-02T10:00:00Z' }),
			flowTask({ id: 'scheduled-created-new', status: '예정', endDate: '', createdAt: '2026-06-09T10:00:00Z' }),
			flowTask({ id: 'scheduled-created-old', status: '예정', endDate: '', createdAt: '2026-06-08T10:00:00Z' })
		];

		expect(sortFlowTaskList(tasks).map((task) => task.id)).toEqual([
			'requested-new',
			'requested-old',
			'scheduled-created-new',
			'scheduled-created-old',
			'stopped',
			'done-new'
		]);
	});
});

function flowSummary(overrides: Partial<FlowSummary>): FlowSummary {
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
		source: 'test',
		...overrides
	};
}

function flowMember(overrides: Partial<FlowMember>): FlowMember {
	return {
		id: 'member-1',
		name: '최견본',
		email: 'lee@example.com',
		role: 'member',
		mattermostStatus: '',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...overrides
	};
}

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: 'task-1',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '',
		type: '기능',
		content: '업무',
		goal: '완료',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		flag: 0,
		...overrides
	};
}
