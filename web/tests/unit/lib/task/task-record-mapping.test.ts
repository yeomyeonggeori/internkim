import { describe, expect, test } from 'bun:test';
import { taskOf, writtenTaskOf } from '../../../../src/lib/task/task-state';
import type { RecordTask } from '../../../../src/lib/task/task-record';
import type { Task } from '../../../../src/routes/task/task-types';

function answered(fields: Partial<RecordTask> = {}): RecordTask {
	return {
		taskID: 'task-1',
		content: '보고서 초안',
		status: 'in_progress',
		size: 'M',
		ownerID: 'member-1',
		ownerName: '이샘플',
		participantIDs: ['member-1'],
		participantNames: ['이샘플'],
		weekCode: '26W36',
		...fields
	};
}

function drafted(fields: Partial<Task> = {}): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '이샘플',
		participantIDs: ['member-1'],
		participantNames: ['이샘플'],
		business: null,
		type: null,
		content: '보고서 초안',
		size: 'M',
		status: 'planned',
		weekCode: '26W36',
		...fields
	};
}

describe('a task the record answered', () => {
	test('carries the parent, the requester and when it was created', () => {
		const task = taskOf(
			answered({
				parentTaskID: 'parent-1',
				requesterID: 'member-2',
				requesterName: '박예시',
				createdAt: '2026-08-11T09:30:00Z'
			})
		);

		expect(task.parentTaskID).toBe('parent-1');
		expect(task.requesterID).toBe('member-2');
		expect(task.requesterName).toBe('박예시');
		expect(task.createdAt).toBe('2026-08-11T09:30:00Z');
	});

	test('reads an empty parent, requester or date as none of one', () => {
		const task = taskOf(answered({ parentTaskID: '', requesterID: '', startDate: '', endDate: '' }));

		expect(task.parentTaskID).toBeUndefined();
		expect(task.requesterID).toBeUndefined();
		expect(task.startDate).toBeUndefined();
		expect(task.endDate).toBeUndefined();
	});

	test('refuses a status the board has no column for', () => {
		expect(() => taskOf(answered({ status: 'blocked' }))).toThrow('unsupported task status: blocked');
	});

	test('takes the days the record settled rather than working them out again', () => {
		const task = taskOf(answered({ startDate: '2026-09-01', endDate: '2026-09-04', weekCode: '26W36' }));

		expect(task.startDate).toBe('2026-09-01');
		expect(task.endDate).toBe('2026-09-04');
		expect(task.weekCode).toBe('26W36');
	});
});

describe('a task the board is writing', () => {
	test('sends the days it holds, and an empty one to take a date off', () => {
		expect(writtenTaskOf(drafted({ startDate: '2026-09-01' }))).toMatchObject({
			startsAt: '2026-09-01',
			endsAt: ''
		});
	});

	test('sends an empty label to file the work under none', () => {
		expect(writtenTaskOf(drafted({ business: null, type: null }))).toMatchObject({ business: '', type: '' });
	});

	test('leaves out a size it does not hold, because no size is not a size', () => {
		expect('size' in writtenTaskOf(drafted({ size: '' }))).toBe(false);
		expect(writtenTaskOf(drafted({ size: 'L' })).size).toBe('L');
	});

	test('gives a task with no title one, because the record refuses a task without', () => {
		expect(writtenTaskOf(drafted({ content: '' })).title).toBe('(제목 없음)');
	});

	test('names everyone taking part, which replaces who was on it', () => {
		expect(writtenTaskOf(drafted({ participantIDs: ['member-1', 'member-2'] })).participantPersonHints).toEqual([
			'member-1',
			'member-2'
		]);
	});
});
