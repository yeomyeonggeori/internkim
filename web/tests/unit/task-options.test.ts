import { describe, expect, test } from 'bun:test';
import { statusOptionsFromSummary } from '../../src/routes/task/task-options';
import type { TaskSummary, Task } from '../../src/routes/task/task-types';

const allStatuses = ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'];
const normalStatuses = ['planned', 'in_progress', 'completed', 'paused', 'stopped'];

describe('flow task status options', () => {
	test('returns the exact normal choices for a central task without requester', () => {
		expect(statusOptionsFromSummary(summary({ source: 'supabase' }), task({ requesterID: '' }))).toEqual(normalStatuses);
	});

	test('returns the exact full-order choices for a central task with requester', () => {
		expect(statusOptionsFromSummary(summary({ source: 'supabase' }), task({ requesterID: 'requester-1' }))).toEqual(allStatuses);
	});

	test('keeps legacy request statuses available for device tasks without requester provenance', () => {
		const deviceSummary = summary({ source: 'sqlite', statusOptions: allStatuses });
		expect(statusOptionsFromSummary(deviceSummary, task({ status: 'requested', requesterID: '' }))).toEqual(allStatuses);
		expect(statusOptionsFromSummary(deviceSummary, task({ status: 'rejected', requesterID: '' }))).toEqual(allStatuses);
	});

	test('preserves the existing global status contract for ordinary device tasks', () => {
		expect(statusOptionsFromSummary(summary({ source: 'sqlite', statusOptions: allStatuses }), task({ status: 'in_progress', requesterID: '' }))).toEqual(allStatuses);
	});
});

function summary(fields: Partial<TaskSummary> = {}): TaskSummary {
	return {
		week: { code: '26W23', startISO: '2026-06-01', endISO: '2026-06-07', previous: '26W22', next: '26W24', isCurrent: true },
		members: [],
		tasks: [],
		metrics: { totalTasks: 0, completedTasks: 0, requestedTasks: 0, pausedTasks: 0, stoppedTasks: 0, statusCounts: {}, businessCounts: {}, typeCounts: {} },
		definitions: { categories: [], types: [], sizes: [] },
		statusOptions: allStatuses,
		currentUserEmail: '',
		currentUserName: '',
		isAdmin: false,
		source: 'test',
		...fields
	};
}

function task(fields: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: '',
		ownerName: '',
		participantIDs: [],
		participantNames: [],
		business: '',
		type: '',
		content: '',
		size: '',
		status: 'planned',
		statusRank: 0,
		weekCode: '',
		...fields
	};
}
