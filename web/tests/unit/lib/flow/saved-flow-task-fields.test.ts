import { describe, expect, test } from 'bun:test';
import { savedFlowTaskFields } from '../../../../src/lib/flow/supabase-flow';
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
