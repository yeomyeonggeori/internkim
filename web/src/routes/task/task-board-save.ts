import { moveTaskOnBoard } from './task-api';
import type { TaskBoardMoveRequest } from './task-board-drag';
import type { LoadTask } from './task-load-tracker';

export type TaskBoardSaveResult = 'saved' | 'saved_with_reload_error' | 'failed';

export type TaskBoardSaveInput = {
	request: TaskBoardMoveRequest;
	week: string;
	currentWeek: () => string;
	loadTask: LoadTask;
	setPageErrorMessage: (message: string) => void;
	saveErrorMessage: string;
	loadErrorMessage: string;
};

export async function saveTaskBoardMove(input: TaskBoardSaveInput): Promise<TaskBoardSaveResult> {
	try {
		await moveTaskOnBoard(input.request);
	} catch (error) {
		if (input.currentWeek() === input.week) {
			input.setPageErrorMessage(errorMessage(error, input.saveErrorMessage));
		}
		return 'failed';
	}

	if (input.currentWeek() !== input.week) return 'saved';

	try {
		const didLoad = await input.loadTask(input.week, { preserveActiveTabOnError: true });
		if (!didLoad && input.currentWeek() === input.week) {
			input.setPageErrorMessage(input.loadErrorMessage);
			return 'saved_with_reload_error';
		}
	} catch (error) {
		if (input.currentWeek() === input.week) {
			input.setPageErrorMessage(errorMessage(error, input.loadErrorMessage));
			return 'saved_with_reload_error';
		}
	}

	return 'saved';
}

function errorMessage(error: unknown, fallbackMessage: string): string {
	return error instanceof Error ? error.message : fallbackMessage;
}
