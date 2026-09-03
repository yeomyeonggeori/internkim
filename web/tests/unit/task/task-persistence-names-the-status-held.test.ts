import { beforeEach, describe, expect, mock, test } from 'bun:test';
import type { Task } from '../../../src/routes/task/task-types';

let asked: { task: Task; statusBefore: string | null | undefined }[] = [];

mock.module('../../../src/routes/task/task-api', () => ({
	saveTask: async (task: Task, _fallbackMessage: string, statusBefore?: string | null) => {
		asked.push({ task, statusBefore });
	},
	deleteTask: async () => undefined
}));

const { saveTaskDraft, updateTaskStatus } = await import('../../../src/routes/task/task-persistence');

function taskWith(fields: Partial<Task> = {}): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '이샘플',
		participantIDs: ['member-2'],
		participantNames: ['박예시'],
		business: null,
		type: null,
		content: '보고서 초안',
		size: 'M',
		status: 'planned',
		statusRank: 0,
		weekCode: '2026-W36',
		...fields
	};
}

const alwaysAllowed = () => true;
const nothingPending = () => false;
const loadsNothing = async () => true;

beforeEach(() => {
	asked = [];
});

describe('what the screen tells the record a task was moved from', () => {
	test('an edit form save names the status the task was opened at', async () => {
		const result = await saveTaskDraft({
			task: taskWith({ status: 'in_progress' }),
			statusBefore: 'planned',
			canUpdateTask: alwaysAllowed,
			loadTask: loadsNothing,
			weekCode: '2026-W36',
			saveErrorMessage: '저장하지 못했습니다.'
		});

		expect(result).toEqual({ status: 'saved' });
		expect(asked).toHaveLength(1);
		expect(asked[0]?.statusBefore).toBe('planned');
		expect(asked[0]?.task.status).toBe('in_progress');
	});

	test('a new task names nothing, because it was not moved from anywhere', async () => {
		await saveTaskDraft({
			task: taskWith({ id: '', status: 'requested' }),
			statusBefore: null,
			canUpdateTask: alwaysAllowed,
			loadTask: loadsNothing,
			weekCode: '2026-W36',
			saveErrorMessage: '저장하지 못했습니다.'
		});

		expect(asked[0]?.statusBefore).toBe(null);
	});

	test('a status changed from the list names the status the row carried', async () => {
		await updateTaskStatus({
			task: taskWith({ status: 'planned' }),
			nextStatus: 'completed',
			canUpdateTask: alwaysAllowed,
			isTaskPending: nothingPending,
			loadTask: loadsNothing,
			weekCode: '2026-W36',
			saveErrorMessage: '저장하지 못했습니다.'
		});

		expect(asked).toHaveLength(1);
		expect(asked[0]?.statusBefore).toBe('planned');
		expect(asked[0]?.task.status).toBe('completed');
	});
});
