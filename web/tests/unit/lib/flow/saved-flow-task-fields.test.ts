import { describe, expect, test } from 'bun:test';
import { dayOf, savedFlowTaskFields } from '../../../../src/lib/flow/supabase-flow';
import type { FlowTask } from '../../../../src/routes/flow/flow-types';

function taskWith(fields: Partial<FlowTask> = {}): FlowTask {
	return {
		id: 'task-1',
		ownerID: '',
		ownerName: '',
		participantIDs: [],
		participantNames: [],
		business: '여명거리',
		type: '기능',
		content: '마켓컬리 CMO 미팅',
		goal: '',
		size: 'XS',
		status: '예정',
		statusRank: 0,
		startDate: '2026-08-20',
		endDate: '2026-08-20',
		weekCode: '26W34',
		flag: 0,
		...fields
	};
}

describe('what the board writes back', () => {
	test('maps every task status to its central status without falling back', () => {
		const mappings = [
			['요청', 'requested'],
			['예정', 'todo'],
			['진행', 'in_progress'],
			['일시정지', 'paused'],
			['중단', 'cancelled'],
			['기각', 'rejected'],
			['완료', 'done']
		] as const;

		for (const [status, storedStatus] of mappings) {
			expect(savedFlowTaskFields(taskWith({ status }), 'update').status).toBe(storedStatus);
		}
	});

	test('includes requester provenance only in central inserts', () => {
		const task = taskWith({ requesterID: 'requester-1', requesterName: '요청자' });

		expect(savedFlowTaskFields(task, 'insert').requester_id).toBe('requester-1');
		expect('requester_id' in savedFlowTaskFields(task, 'update')).toBe(false);
		expect(savedFlowTaskFields(taskWith({ requesterID: '' }), 'insert').requester_id).toBeNull();
	});

	test('gives a task the days the board holds', () => {
		const fields = savedFlowTaskFields(taskWith());
		expect(typeof fields.starts_at).toBe('string');
		expect(typeof fields.ends_at).toBe('string');
	});

	test('leaves an event its own hours, because the board only knows days', () => {
		const fields = savedFlowTaskFields(taskWith({ isEvent: true }));
		expect('starts_at' in fields).toBe(false);
		expect('ends_at' in fields).toBe(false);
	});

	test('still writes everything an event and a task share', () => {
		const fields = savedFlowTaskFields(taskWith({ isEvent: true, content: '팀 회의' }));
		expect(fields.title).toBe('팀 회의');
		expect(fields.business).toBe('여명거리');
		expect(fields.size).toBe('XS');
	});
});

describe('which day the board puts a row on', () => {
	test('reads a Seoul midnight as that Seoul day, not the day before', () => {
		expect(dayOf('2026-06-16 15:00:00+00', 'Asia/Seoul')).toBe('2026-06-17');
		expect(dayOf('2026-08-19T15:00:00+00:00', 'Asia/Seoul')).toBe('2026-08-20');
	});

	test('reads a UTC midnight as that day in Seoul too, so both writers agree', () => {
		expect(dayOf('2026-08-20T00:00:00Z', 'Asia/Seoul')).toBe('2026-08-20');
	});

	test('gives an event the day it is held on rather than the day it is stored on', () => {
		expect(dayOf('2026-08-20 10:00:00+00', 'Asia/Seoul')).toBe('2026-08-20');
	});

	test('answers with nothing when there is no instant', () => {
		expect(dayOf(null, 'Asia/Seoul')).toBeUndefined();
	});
});
