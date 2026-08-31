import { describe, expect, test } from 'bun:test';
import { centralTaskStatusOptions } from '../../../src/lib/task/central-task';
import { whoTaskMoveConcerns as webWhoItConcerns } from '../../../src/lib/server/announce-task';
import {
	centralTaskStatuses,
	whoTaskMoveConcerns as sharedWhoItConcerns
} from '../../../../supabase/functions/_shared/announce-task.ts';

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

describe('the shared task announcement copy tells the same people', () => {
	test('both leave the mover out and count everyone else once', () => {
		expect(sharedWhoItConcerns(task, 'member-two')).toEqual(webWhoItConcerns(task, 'member-two'));
		expect(webWhoItConcerns(task, 'member-two')).toEqual(['member-one', 'member-three']);
	});

	test('both keep a requester nobody participates as', () => {
		const alone = { ...task, task_participant: [] };

		expect(sharedWhoItConcerns(alone, 'member-two')).toEqual(webWhoItConcerns(alone, 'member-two'));
		expect(webWhoItConcerns(alone, 'member-two')).toEqual(['member-one']);
	});

	test('both tell nobody when the mover is the only one it concerns', () => {
		const own = { ...task, requester_id: 'member-one', task_participant: [{ member_id: 'member-one' }] };

		expect(sharedWhoItConcerns(own, 'member-one')).toEqual(webWhoItConcerns(own, 'member-one'));
		expect(webWhoItConcerns(own, 'member-one')).toEqual([]);
	});

	test('both accept a task with no requester', () => {
		const unowned = { ...task, requester_id: null };

		expect(sharedWhoItConcerns(unowned, 'member-three')).toEqual(webWhoItConcerns(unowned, 'member-three'));
		expect(webWhoItConcerns(unowned, 'member-three')).toEqual(['member-two', 'member-one']);
	});
});
