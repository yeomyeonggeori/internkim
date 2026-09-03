import {
	cloneTask,
	createTaskDraft,
	defaultTaskOwner,
	removeTaskParticipant,
	updateTaskParticipantIDs
} from './task-draft';
import { definitionsFromSummary } from './task-options';
import { deleteTaskDraft, saveTaskDraft } from './task-persistence';
import type { LoadTask } from './task-load-tracker';
import {
	canDeleteTask,
	canManageTaskAssignment,
	canRemoveTaskParticipant,
	canUpdateTask,
	currentTaskMember
} from './task-workspace-model';
import { taskText } from './text';
import type { TaskMember, TaskSummary, Task } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

type TaskEditorControllerInput = {
	summary: TaskSummary | null;
	text: TaskPageText;
	loadTask: LoadTask;
	currentWeek: () => string;
	taskWeek: () => string;
	setPageErrorMessage: (message: string) => void;
};

export class TaskEditorController {
	taskDraft = $state<Task | null>(null);
	statusWhenOpened = $state<string | null>(null);
	isEditingTask = $state(false);
	taskErrorMessage = $state('');
	isSavingTask = $state(false);
	isDeletingTask = $state(false);

	private summary = $state<TaskSummary | null>(null);
	private text: TaskPageText = taskText.ko;
	private loadTask: LoadTask;
	private currentWeek: () => string;
	private taskWeek: () => string;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadTask = async () => false;
		this.currentWeek = () => '';
		this.taskWeek = () => '';
		this.setPageErrorMessage = () => {};
	}

	sync = (input: TaskEditorControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadTask = input.loadTask;
		this.currentWeek = input.currentWeek;
		this.taskWeek = input.taskWeek;
		this.setPageErrorMessage = input.setPageErrorMessage;
	};

	openTask = (task: Task, isTaskPending: (taskID: string) => boolean): void => {
		if (isTaskPending(task.id)) return;
		this.taskDraft = cloneTask(task);
		this.statusWhenOpened = task.status;
		this.isEditingTask = false;
		this.taskErrorMessage = '';
	};

	startEditingTask = (): void => {
		if (!this.taskDraft || !this.canUpdateTask(this.taskDraft)) return;
		this.isEditingTask = true;
	};

	createTask = (status?: string, targetParticipantIDs: string[] = [], parentTaskID?: string): void => {
		const owner = this.defaultTaskOwner();
		if (!owner || !this.summary) return;
		this.taskDraft = createTaskDraft(owner, definitionsFromSummary(this.summary), this.taskWeek());
		this.statusWhenOpened = null;
		this.isEditingTask = true;
		if (typeof status === 'string' && status) this.taskDraft.status = status;
		const isRequest = status === 'requested';
		this.taskDraft.requesterID = isRequest ? owner.id : '';
		this.taskDraft.requesterName = isRequest ? owner.name : '';
		if (isRequest) {
			const targetIDs = targetParticipantIDs.length > 0 ? targetParticipantIDs : [owner.id];
			this.taskDraft = updateTaskParticipantIDs(this.taskDraft, this.members(), targetIDs);
		}
		this.taskDraft.parentTaskID = parentTaskID;
		this.taskErrorMessage = '';
	};

	saveTask = async (): Promise<void> => {
		this.isSavingTask = true;
		this.taskErrorMessage = '';
		const result = await saveTaskDraft({
			task: this.taskDraft,
			statusBefore: this.statusWhenOpened,
			canUpdateTask: this.canUpdateTask,
			loadTask: this.loadTask,
			weekCode: this.currentWeek(),
			saveErrorMessage: this.text.task.saveError
		});
		if (result.status === 'saved') this.taskDraft = null;
		if (result.status === 'failed') this.taskErrorMessage = result.errorMessage;
		this.isSavingTask = false;
	};

	deleteTask = async (task: Task): Promise<void> => {
		this.isDeletingTask = true;
		this.taskErrorMessage = '';
		const result = await deleteTaskDraft({
			task,
			canDeleteTask: this.canDeleteTask,
			loadTask: this.loadTask,
			weekCode: this.currentWeek(),
			deleteErrorMessage: this.text.task.deleteError
		});
		if (result.status === 'deleted') this.taskDraft = null;
		if (result.status === 'failed') {
			this.taskErrorMessage = result.errorMessage;
			this.setPageErrorMessage(result.errorMessage);
		}
		this.isDeletingTask = false;
	};

	setParticipantIDs = (memberIDs: string[]): void => {
		if (!this.taskDraft) return;
		this.taskDraft = updateTaskParticipantIDs(this.taskDraft, this.members(), memberIDs);
	};

	removeParticipantID = (memberID: string): void => {
		if (!this.taskDraft || !this.canRemoveParticipant(this.taskDraft, memberID)) return;
		this.taskDraft = removeTaskParticipant(this.taskDraft, memberID);
	};

	closeEditor = (): void => {
		this.taskDraft = null;
		this.isEditingTask = false;
	};

	isOwnTask = (task: Task): boolean => currentTaskMember(this.summary)?.id === task.ownerID;
	canUpdateTask = (task: Task): boolean => canUpdateTask(this.summary, task);
	canDeleteTask = (task: Task): boolean => canDeleteTask(this.summary, task);
	canManageTaskAssignment = (task: Task): boolean => canManageTaskAssignment(this.summary, task);
	canRemoveParticipant = (task: Task, memberID: string): boolean => canRemoveTaskParticipant(task, memberID);

	private members(): TaskMember[] {
		return this.summary?.members ?? [];
	}

	private defaultTaskOwner(): TaskMember | undefined {
		return defaultTaskOwner(this.members(), this.summary?.currentUserEmail ?? '', '');
	}
}
