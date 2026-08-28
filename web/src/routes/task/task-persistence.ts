import { deleteTask, saveTask } from './task-api';
import type { LoadTask } from './task-load-tracker';
import type { Task } from './task-types';

type TaskSaveResult =
	| { status: 'ignored' }
	| { status: 'saved' }
	| { status: 'failed'; errorMessage: string };

type TaskDeleteResult =
	| { status: 'ignored' }
	| { status: 'deleted' }
	| { status: 'failed'; errorMessage: string };

type TaskStatusResult =
	| { status: 'ignored' }
	| { status: 'saved' }
	| { status: 'failed'; errorMessage: string };

type SaveTaskDraftInput = {
	task: Task | null;
	canUpdateTask: (task: Task) => boolean;
	loadTask: LoadTask;
	weekCode: string;
	saveErrorMessage: string;
};

type DeleteTaskInput = {
	task: Task;
	canDeleteTask: (task: Task) => boolean;
	loadTask: LoadTask;
	weekCode: string;
	deleteErrorMessage: string;
};

type UpdateTaskStatusInput = {
	task: Task;
	nextStatus: string;
	canUpdateTask: (task: Task) => boolean;
	isTaskPending: (taskID: string) => boolean;
	loadTask: LoadTask;
	weekCode: string;
	saveErrorMessage: string;
};

export async function saveTaskDraft(input: SaveTaskDraftInput): Promise<TaskSaveResult> {
	if (!input.task || !input.canUpdateTask(input.task)) return { status: 'ignored' };
	try {
		await saveTask(input.task, input.saveErrorMessage);
		await input.loadTask(input.weekCode);
		return { status: 'saved' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.saveErrorMessage) };
	}
}

export async function deleteTaskDraft(input: DeleteTaskInput): Promise<TaskDeleteResult> {
	if (!input.task.id || !input.canDeleteTask(input.task)) return { status: 'ignored' };
	try {
		await deleteTask(input.task.id, input.deleteErrorMessage);
		await input.loadTask(input.weekCode);
		return { status: 'deleted' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.deleteErrorMessage) };
	}
}

export async function updateTaskStatus(input: UpdateTaskStatusInput): Promise<TaskStatusResult> {
	if (!input.task.id || input.nextStatus === input.task.status) return { status: 'ignored' };
	if (!input.canUpdateTask(input.task)) return { status: 'ignored' };
	if (input.isTaskPending(input.task.id)) return { status: 'ignored' };
	try {
		await saveTask({ ...input.task, status: input.nextStatus }, input.saveErrorMessage);
		await input.loadTask(input.weekCode);
		return { status: 'saved' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.saveErrorMessage) };
	}
}

function errorMessageFrom(error: unknown, fallbackMessage: string): string {
	return error instanceof Error ? error.message : fallbackMessage;
}
