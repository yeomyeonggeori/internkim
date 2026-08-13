import { describe, expect, test } from 'bun:test';
import {
	centralFlowTaskFromRow,
	centralFlowTaskSelection
} from '../../../../src/lib/flow/central-flow-task';

const statuses = [
	['requested', '요청'],
	['todo', '예정'],
	['in_progress', '진행'],
	['paused', '일시정지'],
	['cancelled', '중단'],
	['rejected', '기각'],
	['done', '완료']
] as const;

describe('central flow task rows', () => {
	test('selects requester provenance with participants', () => {
		expect(centralFlowTaskSelection).toBe(
			'id, title, status, note, business, type, size, starts_at, ends_at, due_at, is_event, updated_at, requester_id, requester_name, was_requested, task_participant (member_id)'
		);
	});

	test('maps every central status without losing request state', () => {
		for (const [status, flowStatus] of statuses) {
			expect(centralFlowTaskFromRow(row({ status }), names, dayOf).status).toBe(flowStatus);
		}
	});

	test('rejects an unknown inbound central status', () => {
		expect(() => centralFlowTaskFromRow(row({ status: 'blocked' }), names, dayOf)).toThrow(
			'unsupported central task status: blocked'
		);
	});

	test('maps requester identity and display name', () => {
		const task = centralFlowTaskFromRow(row({ requester_id: 'requester-1' }), names, dayOf);

		expect(task.requesterID).toBe('requester-1');
		expect(task.requesterName).toBe('요청자');
	});

	test('maps durable requester provenance after the requester leaves', () => {
		const task = centralFlowTaskFromRow(row({
			requester_id: null,
			requester_name: '퇴사한 요청자',
			was_requested: true
		}), names, dayOf);

		expect(task.wasRequested).toBe(true);
		expect(task.requesterID).toBe('');
		expect(task.requesterName).toBe('퇴사한 요청자');
	});

	test('derives compatibility owner only for one participant', () => {
		const none = centralFlowTaskFromRow(row({ task_participant: [] }), names, dayOf);
		const one = centralFlowTaskFromRow(row({ task_participant: [{ member_id: 'target-1' }] }), names, dayOf);
		const two = centralFlowTaskFromRow(row({
			task_participant: [{ member_id: 'target-1' }, { member_id: 'target-2' }]
		}), names, dayOf);

		expect([none.ownerID, none.ownerName]).toEqual(['', '']);
		expect([one.ownerID, one.ownerName]).toEqual(['target-1', '대상자']);
		expect([two.ownerID, two.ownerName]).toEqual(['', '']);
	});
});

const names = new Map([
	['requester-1', '요청자'],
	['target-1', '대상자'],
	['target-2', '다른 대상자']
]);

function dayOf(value: string | null): string | undefined {
	return value?.slice(0, 10);
}

function row(overrides: Record<string, unknown> = {}) {
	return {
		id: 'task-1',
		title: '업무',
		status: 'todo' as const,
		note: null,
		business: null,
		type: null,
		size: null,
		is_event: false,
		starts_at: null,
		ends_at: null,
		due_at: null,
		updated_at: '2026-08-13T00:00:00Z',
		requester_id: null,
		requester_name: null,
		was_requested: false,
		task_participant: [],
		...overrides
	};
}
