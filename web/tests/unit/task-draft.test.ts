import { describe, expect, test } from 'bun:test';
import {
	createTaskDraft,
	removeTaskParticipant,
	toggleTaskParticipantID,
	updateTaskParticipantIDs
} from '../../src/routes/task/task-draft';
import type { TaskMember, Task } from '../../src/routes/task/task-types';

describe('a new task draft', () => {
	test('leaves business, type and size for the record to decide', () => {
		const owner: TaskMember = { id: 'member-1', name: '이샘플', email: 'sample@example.com', role: 'member', activeTaskCount: 0, completeTaskCount: 0 };
		const draft = createTaskDraft(owner, { categories: ['영업'], types: ['문서'], sizes: [] }, '26W39');
		expect(draft.business).toBeNull();
		expect(draft.type).toBeNull();
		expect(draft.size).toBe('');
	});
});

describe('flow task participant draft', () => {
	test('keeps duplicate display names distinct by canonical member ID', () => {
		const updated = updateTaskParticipantIDs(
			task(),
			[
				member({ id: 'left', name: '동명이인', email: 'left@example.com' }),
				member({ id: 'right', name: '동명이인', email: 'right@example.com' })
			],
			['left', 'right']
		);

		expect(updated.participantIDs).toEqual(['left', 'right']);
		expect(updated.participantNames).toEqual(['동명이인', '동명이인']);
		expect(updated.ownerID).toBe('');
	});

	test('drops the participant asked for and lets the record name the owner again', () => {
		const original = task({
			ownerID: 'owner',
			ownerName: '담당자',
			participantIDs: ['owner', 'participant'],
			participantNames: ['담당자', '참여자']
		});

		expect(removeTaskParticipant(original, 'participant').participantIDs).toEqual(['owner']);
		expect(removeTaskParticipant(original, 'owner').participantIDs).toEqual(['participant']);
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
		weekCode: '26W23',
		...fields
	};
}
