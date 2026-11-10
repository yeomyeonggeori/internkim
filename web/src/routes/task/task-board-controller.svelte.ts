import { feelHaptic } from '$lib/native-shell/haptics';
import { createTaskBoardMove, type TaskBoardMoveRequest } from './task-board-drag';
import { saveTaskBoardMove } from './task-board-save';
import type { LoadTask } from './task-load-tracker';
import { isTaskStatusCompleted, isTaskStatusRequested } from './task-status';
import { buildTaskChildProgressByParent } from './task-relationships';
import { canUpdateTask } from './task-workspace-model';
import { taskText } from './text';
import type { Task, TaskSummary } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

export type ChildCompletionChoice = 'with_children' | 'alone' | 'cancelled';

export type ChildCompletionQuestion = {
	taskTitle: string;
	unfinishedChildren: number;
	answer: (choice: ChildCompletionChoice) => void;
};

type TaskBoardControllerInput = {
	text: TaskPageText;
	loadTask: LoadTask;
	currentWeek: () => string;
	getSummary: () => TaskSummary | null;
	setSummary: (summary: TaskSummary) => void;
	setPageErrorMessage: (message: string) => void;
	announceMove?: (message: string) => void;
	announceMoveFailure?: (message: string) => void;
};

export class TaskBoardController {
	pendingTaskIDs = $state<string[]>([]);
	childCompletionQuestion = $state<ChildCompletionQuestion | null>(null);

	private text: TaskPageText = taskText.ko;
	private loadTask: LoadTask;
	private currentWeek: () => string;
	private getSummary: () => TaskSummary | null;
	private setSummary: (summary: TaskSummary) => void;
	private setPageErrorMessage: (message: string) => void;
	private announceMove: (message: string) => void = () => {};
	private announceMoveFailure: (message: string) => void = () => {};

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
		this.announceMoveFailure = input.announceMoveFailure ?? this.setPageErrorMessage;
	};

	moveTaskOnBoard = async (request: TaskBoardMoveRequest): Promise<void> => {
		const asked = this.getSummary();
		if (!asked || this.pendingTaskIDs.length > 0 || this.childCompletionQuestion) return;
		const askedTask = asked.tasks.find((candidate) => candidate.id === request.taskID);
		if (!askedTask || !canUpdateTask(askedTask)) return;
		if (isTaskStatusRequested(request.targetStatus) && !askedTask.requesterID) {
			this.announceMoveFailure(this.text.task.requestNeedsRequester);
			return;
		}
		const completeChildren = await this.whetherChildrenComplete(askedTask, request.targetStatus, asked);
		if (completeChildren === null) return;

		const summary = this.getSummary();
		if (!summary || this.pendingTaskIDs.length > 0) return;
		const task = summary.tasks.find((candidate) => candidate.id === request.taskID);
		if (!task) return;
		const move = createTaskBoardMove(summary.tasks, request);
		if (!move) return;

		const previousSummary = summary;
		const week = this.currentWeek();
		this.pendingTaskIDs = move.updates.map((task) => task.id);
		this.setPageErrorMessage('');
		this.setSummary({ ...summary, tasks: move.tasks });

		try {
			const saveResult = await saveTaskBoardMove({
				request: { ...request, completeChildren },
				week,
				currentWeek: this.currentWeek,
				loadTask: this.loadTask,
				setPageErrorMessage: (message) => {
					if (message) this.announceMoveFailure(message);
				},
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

	private async whetherChildrenComplete(task: Task, targetStatus: string, summary: TaskSummary): Promise<boolean | null> {
		if (!isTaskStatusCompleted(targetStatus)) return false;
		const unfinishedChildren = unfinishedChildrenOf(task, summary);
		if (unfinishedChildren === 0) return false;
		const choice = await new Promise<ChildCompletionChoice>((answer) => {
			this.childCompletionQuestion = { taskTitle: task.content, unfinishedChildren, answer };
		});
		this.childCompletionQuestion = null;
		if (choice === 'cancelled') return null;
		return choice === 'with_children';
	}
}

function unfinishedChildrenOf(task: Task, summary: TaskSummary): number {
	const progress = summary.childProgressByParent?.[task.id] ?? buildTaskChildProgressByParent(summary.tasks).get(task.id);
	return progress ? progress.total - progress.completed : 0;
}
