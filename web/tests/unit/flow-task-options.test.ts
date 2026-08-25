import { describe, expect, test } from 'bun:test';
import { statusOptionsFromSummary } from '../../src/routes/flow/flow-task-options';
import type { FlowSummary, FlowTask } from '../../src/routes/flow/flow-types';

const allStatuses = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'];
const normalStatuses = ['예정', '진행', '완료', '일시정지', '중단'];

describe('flow task status options', () => {
	test('returns the exact normal choices for a central task without requester', () => {
		expect(statusOptionsFromSummary(summary({ source: 'supabase' }), task({ requesterID: '' }))).toEqual(normalStatuses);
	});

	test('returns the exact full-order choices for a central task with requester', () => {
		expect(statusOptionsFromSummary(summary({ source: 'supabase' }), task({ requesterID: 'requester-1' }))).toEqual(allStatuses);
	});

	test('keeps legacy request statuses available for device tasks without requester provenance', () => {
		const deviceSummary = summary({ source: 'sqlite', statusOptions: allStatuses });
		expect(statusOptionsFromSummary(deviceSummary, task({ status: '요청', requesterID: '' }))).toEqual(allStatuses);
		expect(statusOptionsFromSummary(deviceSummary, task({ status: '기각', requesterID: '' }))).toEqual(allStatuses);
	});

	test('preserves the existing global status contract for ordinary device tasks', () => {
		expect(statusOptionsFromSummary(summary({ source: 'sqlite', statusOptions: allStatuses }), task({ status: '진행', requesterID: '' }))).toEqual(allStatuses);
	});
});

function summary(fields: Partial<FlowSummary> = {}): FlowSummary {
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

function task(fields: Partial<FlowTask>): FlowTask {
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
		status: '예정',
		statusRank: 0,
		weekCode: '',
		...fields
	};
}
