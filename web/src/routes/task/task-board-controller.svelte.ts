import { feelHaptic } from '$lib/native-shell/haptics';
import { createTaskBoardMove, type TaskBoardMoveRequest } from './task-board-drag';
import { saveTaskBoardMove } from './task-board-save';
import type { LoadTask } from './task-load-tracker';
import { isTaskStatusCompleted } from './task-status';
import { canUpdateTask } from './task-workspace-model';
import { taskText } from './text';
import type { TaskSummary } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

type TaskBoardControllerInput = {
	text: TaskPageText;
	loadTask: LoadTask;
	currentWeek: () => string;
	getSummary: () => TaskSummary | null;
	setSummary: (summary: TaskSummary) => void;
	setPageErrorMessage: (message: string) => void;
	announceMove?: (message: string) => void;
};

export class TaskBoardController {
	pendingTaskIDs = $state<string[]>([]);

	private text: TaskPageText = taskText.ko;
	private loadTask: LoadTask;
	private currentWeek: () => string;
	private getSummary: () => TaskSummary | null;
	private setSummary: (summary: TaskSummary) => void;
	private setPageErrorMessage: (message: string) => void;
	private announceMove: (message: string) => void = () => {};

	constructor() {
		this.loadTask = async () => false;
		this.currentWeek = () => '';
		this.getSummary = () => null;
		this.setSummary = () => {};
		this.setPageErrorMessage = () => {};
	}

	sync = (input: TaskBoardControllerInput): void => {
		this.text = input.text;
		this.loadTask = input.loadTask;
		this.currentWeek = input.currentWeek;
		this.getSummary = input.getSummary;
		this.setSummary = input.setSummary;
		this.setPageErrorMessage = input.setPageErrorMessage;
		this.announceMove = input.announceMove ?? (() => {});
	};

	moveTaskOnBoard = async (request: TaskBoardMoveRequest): Promise<void> => {
		const summary = this.getSummary();
		if (!summary || this.pendingTaskIDs.length > 0) return;
		const task = summary.tasks.find((candidate) => candidate.id === request.taskID);
		if (!task || !canUpdateTask(summary, task)) return;
		const move = createTaskBoardMove(summary.tasks, request);
		if (!move) return;

		const previousSummary = summary;
		const week = this.currentWeek();
		this.pendingTaskIDs = move.updates.map((task) => task.id);
		this.setPageErrorMessage('');
		this.setSummary({ ...summary, tasks: move.tasks });

		try {
			const saveResult = await saveTaskBoardMove({
				request,
				week,
				currentWeek: this.currentWeek,
				loadTask: this.loadTask,
				setPageErrorMessage: this.setPageErrorMessage,
				saveErrorMessage: this.text.task.saveError,
				loadErrorMessage: this.text.loadError
			});
			if (saveResult === 'failed' && this.currentWeek() === week) {
				this.setSummary(previousSummary);
			}
			if (saveResult !== 'failed') {
				if (isTaskStatusCompleted(request.targetStatus)) feelHaptic('success');
				const statusLabels: Record<string, string> = this.text.status;
				this.announceMove(`${task.content} → ${statusLabels[request.targetStatus] ?? request.targetStatus}`);
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
