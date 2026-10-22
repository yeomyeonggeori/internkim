import { describe, expect, mock, test } from 'bun:test';
import type { Task } from '../../../src/routes/task/task-types';

mock.module('../../../src/routes/task/task-api', () => ({
	saveTask: async () => undefined,
	deleteTask: async () => undefined
}));

const { deleteTaskDraft, saveTaskDraft } = await import('../../../src/routes/task/task-persistence');

function taskWith(fields: Partial<Task> = {}): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '이샘플',
		participantIDs: [],
		participantNames: [],
		business: null,
		type: null,
		content: '보고서 초안',
		size: 'M',
		status: 'planned',
		weekCode: '2026-W36',
		...fields
	};
}

const alwaysAllowed = () => true;
const reloadsForever = () => new Promise<boolean>(() => {});
const stillWaiting = 'still waiting for the reload';

function settledBeforeTheReload<T>(promise: Promise<T>): Promise<T | typeof stillWaiting> {
	return Promise.race([promise, new Promise<typeof stillWaiting>((resolve) => setTimeout(() => resolve(stillWaiting), 50))]);
}

describe('the editor closes once the record answers, without waiting for the list to reload', () => {
	test('a new task is saved while the week is still reloading', async () => {
		const result = await settledBeforeTheReload(
			saveTaskDraft({
				task: taskWith({ id: '' }),
				statusBefore: null,
				canUpdateTask: alwaysAllowed,
				loadTask: reloadsForever,
				weekCode: '2026-W36',
				saveErrorMessage: '저장하지 못했습니다.'
			})
		);

		expect(result).toEqual({ status: 'saved' });
	});

	test('a deleted task is gone while the week is still reloading', async () => {
		const result = await settledBeforeTheReload(
			deleteTaskDraft({
				task: taskWith(),
				canDeleteTask: alwaysAllowed,
				loadTask: reloadsForever,
				weekCode: '2026-W36',
				deleteErrorMessage: '삭제하지 못했습니다.'
			})
		);

		expect(result).toEqual({ status: 'deleted' });
	});
});
