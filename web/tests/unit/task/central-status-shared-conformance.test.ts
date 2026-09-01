import { describe, expect, test } from 'bun:test';
import { centralTaskStatusOptions } from '../../../src/lib/task/central-task';
import { centralTaskStatuses } from '../../../../supabase/functions/_shared/central-task-status.ts';
import { whoTaskMoveConcerns } from '../../../../supabase/functions/_shared/announce-task.ts';

const task = {
	id: 'task-one',
	title: '이샘플 인수인계',
	status: 'in_progress',
	requester_id: 'member-one',
	task_participant: [{ member_id: 'member-two' }, { member_id: 'member-one' }, { member_id: 'member-three' }]
};

describe('the shared task status copy stays interchangeable with the web one', () => {
	test('both name the same statuses', () => {
		expect([...centralTaskStatuses].sort()).toEqual([...centralTaskStatusOptions].sort());
	});
});

describe('a task move concerns everyone but whoever moved it', () => {
	test('the mover is left out and everyone else is counted once', () => {
		expect(whoTaskMoveConcerns(task, 'member-two')).toEqual(['member-one', 'member-three']);
	});

	test('a requester nobody participates as is still told', () => {
		expect(whoTaskMoveConcerns({ ...task, task_participant: [] }, 'member-two')).toEqual([
			'member-one'
		]);
	});

	test('nobody is told when the mover is the only one it concerns', () => {
		const own = { ...task, requester_id: 'member-one', task_participant: [{ member_id: 'member-one' }] };

		expect(whoTaskMoveConcerns(own, 'member-one')).toEqual([]);
	});

	test('a task with no requester tells its participants', () => {
		expect(whoTaskMoveConcerns({ ...task, requester_id: null }, 'member-three')).toEqual([
			'member-two',
			'member-one'
		]);
	});
});
