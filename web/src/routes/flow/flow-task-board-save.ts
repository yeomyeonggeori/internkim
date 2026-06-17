import { moveFlowTaskOnBoard } from './flow-api';
import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import type { LoadFlow } from './flow-load-tracker';

export type FlowTaskBoardSaveResult = 'saved' | 'saved_with_reload_error' | 'failed';

export type FlowTaskBoardSaveInput = {
	request: FlowTaskBoardMoveRequest;
	week: string;
	currentWeek: () => string;
	loadFlow: LoadFlow;
	setPageErrorMessage: (message: string) => void;
	saveErrorMessage: string;
	loadErrorMessage: string;
};

export async function saveFlowTaskBoardMove(input: FlowTaskBoardSaveInput): Promise<FlowTaskBoardSaveResult> {
	try {
		await moveFlowTaskOnBoard(input.request, input.saveErrorMessage);
	} catch (error) {
		if (input.currentWeek() === input.week) {
			input.setPageErrorMessage(errorMessage(error, input.saveErrorMessage));
		}
		return 'failed';
	}

	if (input.currentWeek() !== input.week) return 'saved';

	try {
		const didLoad = await input.loadFlow(input.week, { preserveActiveTabOnError: true });
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
