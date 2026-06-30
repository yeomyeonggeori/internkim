import { createFlowTaskBoardMove, type FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import { saveFlowTaskBoardMove } from './flow-task-board-save';
import type { LoadFlow } from './flow-load-tracker';
import { canUpdateFlowTask } from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowSummary } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTaskBoardControllerInput = {
	text: FlowPageText;
	loadFlow: LoadFlow;
	currentWeek: () => string;
	getSummary: () => FlowSummary | null;
	setSummary: (summary: FlowSummary) => void;
	setPageErrorMessage: (message: string) => void;
};

export class FlowTaskBoardController {
	pendingTaskIDs = $state<string[]>([]);

	private text: FlowPageText = flowText.ko;
	private loadFlow: LoadFlow;
	private currentWeek: () => string;
	private getSummary: () => FlowSummary | null;
	private setSummary: (summary: FlowSummary) => void;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadFlow = async () => false;
		this.currentWeek = () => '';
		this.getSummary = () => null;
		this.setSummary = () => {};
		this.setPageErrorMessage = () => {};
	}

	sync = (input: FlowTaskBoardControllerInput): void => {
		this.text = input.text;
		this.loadFlow = input.loadFlow;
		this.currentWeek = input.currentWeek;
		this.getSummary = input.getSummary;
		this.setSummary = input.setSummary;
		this.setPageErrorMessage = input.setPageErrorMessage;
	};

	moveTaskOnBoard = async (request: FlowTaskBoardMoveRequest): Promise<void> => {
		const summary = this.getSummary();
		if (!summary || this.pendingTaskIDs.length > 0) return;
		const task = summary.tasks.find((candidate) => candidate.id === request.taskID);
		if (!task || !canUpdateFlowTask(summary, task)) return;
		const move = createFlowTaskBoardMove(summary.tasks, request);
		if (!move) return;

		const previousSummary = summary;
		const week = this.currentWeek();
		this.pendingTaskIDs = move.updates.map((task) => task.id);
		this.setPageErrorMessage('');
		this.setSummary({ ...summary, tasks: move.tasks });

		try {
			const saveResult = await saveFlowTaskBoardMove({
				request,
				week,
				currentWeek: this.currentWeek,
				loadFlow: this.loadFlow,
				setPageErrorMessage: this.setPageErrorMessage,
				saveErrorMessage: this.text.task.saveError,
				loadErrorMessage: this.text.loadError
			});
			if (saveResult === 'failed' && this.currentWeek() === week) {
				this.setSummary(previousSummary);
			}
			if (saveResult === 'saved_with_reload_error' && this.currentWeek() === week) {
				this.setSummary({ ...previousSummary, tasks: move.tasks });
			}
		} finally {
			this.pendingTaskIDs = [];
		}
	};

	isTaskPending = (taskID: string): boolean => this.pendingTaskIDs.includes(taskID);
}
