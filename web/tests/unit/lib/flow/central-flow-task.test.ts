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
	test('selects requester identity with participants', () => {
		expect(centralFlowTaskSelection).toBe(
			'id, parent_task_id, title, status, note, business, type, size, starts_at, ends_at, due_at, is_event, updated_at, requester_id, requester:member!task_requester_id_fkey (name, email), task_participant (member_id)'
		);
	});

	test('maps the parent relationship', () => {
		expect(centralFlowTaskFromRow(row({ parent_task_id: 'parent-1' }), names, dayOf).parentTaskID).toBe('parent-1');
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
		const task = centralFlowTaskFromRow(row({
			requester_id: 'requester-1',
			requester: { name: '요청자', email: 'requester@example.com' }
		}), names, dayOf);

		expect(task.requesterID).toBe('requester-1');
		expect(task.requesterName).toBe('요청자');
	});

	test('maps requester identity from the Supabase inferred array shape', () => {
		const task = centralFlowTaskFromRow(row({
			requester_id: 'requester-1',
			requester: [{ name: '배열 요청자', email: 'requester@example.com' }]
		}), new Map(), dayOf);

		expect(task.requesterID).toBe('requester-1');
		expect(task.requesterName).toBe('배열 요청자');
	});

	test('prefers the freshly loaded member name over a cached requester join', () => {
		const task = centralFlowTaskFromRow(row({
			requester_id: 'requester-1',
			requester: { name: '이전 이름', email: 'requester@example.com' }
		}), names, dayOf);

		expect(task.requesterName).toBe('요청자');
	});

	test('uses requester email when the joined member has no name', () => {
		const task = centralFlowTaskFromRow(row({
			requester_id: 'requester-1',
			requester: { name: '', email: 'requester@example.com' }
		}), new Map(), dayOf);

		expect(task.requesterName).toBe('requester@example.com');
	});

	test('keeps the requester id when the joined member has no display identity', () => {
		const task = centralFlowTaskFromRow(row({
			requester_id: 'requester-1',
			requester: { name: null, email: null }
		}), new Map(), dayOf);

		expect(task.requesterID).toBe('requester-1');
		expect(task.requesterName).toBe('');
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
		parent_task_id: null,
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
		requester: null,
		task_participant: [],
		...overrides
	};
}
