import { describe, expect, test } from 'bun:test';
import {
	removeTaskParticipant,
	toggleTaskParticipantID,
	updateTaskParticipantIDs
} from '../../src/routes/task/task-draft';
import type { TaskMember, Task } from '../../src/routes/task/task-types';

describe('flow task participant draft', () => {
	test('keeps duplicate display names distinct by canonical member ID', () => {
		const updated = updateTaskParticipantIDs(
			task(),
			[
				member({ id: 'left', name: '동명이인', email: 'left@example.com' }),
				member({ id: 'right', name: '동명이인', email: 'right@example.com' })
			],
			['left', 'right'],
			'supabase'
		);

		expect(updated.participantIDs).toEqual(['left', 'right']);
		expect(updated.participantNames).toEqual(['동명이인', '동명이인']);
		expect(updated.ownerID).toBe('');
	});

	test('keeps a legacy device owner first and nonempty when participants change', () => {
		const updated = updateTaskParticipantIDs(
			task({
				ownerID: 'owner',
				ownerName: '담당자',
				participantIDs: ['owner'],
				participantNames: ['담당자']
			}),
			[
				member({ id: 'owner', name: '담당자' }),
				member({ id: 'participant', name: '참여자' })
			],
			['participant'],
			'sqlite'
		);

		expect(updated.ownerID).toBe('owner');
		expect(updated.ownerName).toBe('담당자');
		expect(updated.participantIDs).toEqual(['owner', 'participant']);
		expect(updated.participantNames).toEqual(['담당자', '참여자']);
	});

	test('does not remove the legacy device owner from a task draft', () => {
		const original = task({
			ownerID: 'owner',
			ownerName: '담당자',
			participantIDs: ['owner', 'participant'],
			participantNames: ['담당자', '참여자']
		});

		expect(removeTaskParticipant(original, 'owner', 'sqlite').participantIDs).toEqual(['owner', 'participant']);
		expect(removeTaskParticipant(original, 'participant', 'sqlite').participantIDs).toEqual(['owner']);
	});

	test('does not toggle away a selected participant without removal authority', () => {
		const original = task({ participantIDs: ['owner'], participantNames: ['담당자'] });

		expect(toggleTaskParticipantID(original, 'owner', false)).toEqual(['owner']);
		expect(toggleTaskParticipantID(original, 'owner', true)).toEqual([]);
		expect(toggleTaskParticipantID(original, 'participant', false)).toEqual(['owner', 'participant']);
	});
});

function member(fields: Partial<TaskMember>): TaskMember {
	return {
		id: 'member',
		name: '구성원',
		email: 'member@example.com',
		role: 'member',
		mattermostStatus: '',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...fields
	};
}

function task(fields: Partial<Task> = {}): Task {
	return {
		id: 'task',
		ownerID: '',
		ownerName: '',
		participantIDs: [],
		participantNames: [],
		business: '',
		type: '',
		content: '업무',
		size: 'M',
		status: 'planned',
		statusRank: 0,
		weekCode: '26W23',
		...fields
	};
}
