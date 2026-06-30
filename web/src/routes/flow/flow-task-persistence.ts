import { deleteFlowTask, saveFlowTask } from './flow-api';
import type { LoadFlow } from './flow-load-tracker';
import type { FlowTask } from './flow-types';

type FlowTaskSaveResult =
	| { status: 'ignored' }
	| { status: 'saved' }
	| { status: 'failed'; errorMessage: string };

type FlowTaskDeleteResult =
	| { status: 'ignored' }
	| { status: 'deleted' }
	| { status: 'failed'; errorMessage: string };

type FlowTaskStatusResult =
	| { status: 'ignored' }
	| { status: 'saved' }
	| { status: 'failed'; errorMessage: string };

type SaveFlowTaskDraftInput = {
	task: FlowTask | null;
	canUpdateTask: (task: FlowTask) => boolean;
	loadFlow: LoadFlow;
	weekCode: string;
	saveErrorMessage: string;
};

type DeleteFlowTaskInput = {
	task: FlowTask;
	canDeleteTask: (task: FlowTask) => boolean;
	loadFlow: LoadFlow;
	weekCode: string;
	deleteErrorMessage: string;
};

type UpdateFlowTaskStatusInput = {
	task: FlowTask;
	nextStatus: string;
	canUpdateTask: (task: FlowTask) => boolean;
	isTaskPending: (taskID: string) => boolean;
	loadFlow: LoadFlow;
	weekCode: string;
	saveErrorMessage: string;
};

export async function saveFlowTaskDraft(input: SaveFlowTaskDraftInput): Promise<FlowTaskSaveResult> {
	if (!input.task || !input.canUpdateTask(input.task)) return { status: 'ignored' };
	try {
		await saveFlowTask(input.task, input.saveErrorMessage);
		await input.loadFlow(input.weekCode);
		return { status: 'saved' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.saveErrorMessage) };
	}
}

export async function deleteFlowTaskDraft(input: DeleteFlowTaskInput): Promise<FlowTaskDeleteResult> {
	if (!input.task.id || !input.canDeleteTask(input.task)) return { status: 'ignored' };
	try {
		await deleteFlowTask(input.task.id, input.deleteErrorMessage);
		await input.loadFlow(input.weekCode);
		return { status: 'deleted' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.deleteErrorMessage) };
	}
}

export async function updateFlowTaskStatus(input: UpdateFlowTaskStatusInput): Promise<FlowTaskStatusResult> {
	if (!input.task.id || input.nextStatus === input.task.status) return { status: 'ignored' };
	if (!input.canUpdateTask(input.task)) return { status: 'ignored' };
	if (input.isTaskPending(input.task.id)) return { status: 'ignored' };
	try {
		await saveFlowTask({ ...input.task, status: input.nextStatus }, input.saveErrorMessage);
		await input.loadFlow(input.weekCode);
		return { status: 'saved' };
	} catch (error) {
		return { status: 'failed', errorMessage: errorMessageFrom(error, input.saveErrorMessage) };
	}
}

function errorMessageFrom(error: unknown, fallbackMessage: string): string {
	return error instanceof Error ? error.message : fallbackMessage;
}
