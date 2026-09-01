import { isEmbeddedFrame, openDetailWindow } from '$lib/embedded';
import type { TaskBoardMoveRequest } from './task-board-drag';
import { TaskBoardController } from './task-board-controller.svelte';
import { TaskEditorController } from './task-editor-controller.svelte';
import { TaskFiltersController } from './task-filters-controller.svelte';
import { taskBoardParticipantScope } from './task-board-participant-scope';
import { taskBusinessColor, taskTypeColor } from './task-definition-colors';
import { updateTaskStatus } from './task-persistence';
import { TaskQuickCreateController } from './task-quick-create-controller.svelte';
import { TaskRelationshipController } from './task-relationship-controller.svelte';
import type { LoadTask } from './task-load-tracker';
import {
	categorySelectOptions,
	definitionsFromSummary,
	taskStatusLabel,
	sizeSelectOptions,
	statusOptionsFromSummary,
	statusSelectOptions,
	typeSelectOptions
} from './task-options';
import {
	canUpdateTask,
	currentTaskMember,
} from './task-workspace-model';
import { taskText } from './text';
import type { TaskQuickTaskCreateResult, TaskSummary, Task } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

type TasksControllerInput = {
	summary: TaskSummary | null;
	text: TaskPageText;
	loadTask: LoadTask;
	setPageErrorMessage: (message: string) => void;
	announceMove?: (message: string) => void;
};

export function createTasksController() {
	return new TasksController();
}

class TasksController {
	summary = $state<TaskSummary | null>(null);
	text: TaskPageText = taskText.ko;
	board = new TaskBoardController();
	editor = new TaskEditorController();
	filters = new TaskFiltersController();
	quickTask = new TaskQuickCreateController();
	relationships = new TaskRelationshipController();
	pendingStatusTaskID = $state('');

	private loadTask: LoadTask;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadTask = async () => false;
		this.setPageErrorMessage = () => {};
	}

	get taskDraft(): Task | null {
		return this.editor.taskDraft;
	}

	set taskDraft(taskDraft: Task | null) {
		this.editor.taskDraft = taskDraft;
	}

	get taskErrorMessage(): string {
		return this.editor.taskErrorMessage;
	}

	get isSavingTask(): boolean {
		return this.editor.isSavingTask;
	}

	get isDeletingTask(): boolean {
		return this.editor.isDeletingTask;
	}

	sync = (input: TasksControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadTask = input.loadTask;
		this.setPageErrorMessage = input.setPageErrorMessage;
		this.filters.sync(this.summary);
		this.quickTask.sync({
			summary: this.summary,
			text: this.text,
			loadTask: this.loadTask
		});
		this.board.sync({
			text: this.text,
			loadTask: this.loadTask,
			currentWeek: this.currentWeek,
			getSummary: () => this.summary,
			setSummary: (summary) => {
				this.summary = summary;
			},
			setPageErrorMessage: this.setPageErrorMessage,
			announceMove: input.announceMove
		});
		this.editor.sync({
			summary: this.summary,
			text: this.text,
			loadTask: this.loadTask,
			currentWeek: this.currentWeek,
			taskWeek: this.taskWeek,
			setPageErrorMessage: this.setPageErrorMessage
		});
		this.relationships.sync({
			loadTask: this.loadTask,
			weekCode: this.currentWeek,
			fallbackMessage: () => this.text.task.relationships.updateError,
			setErrorMessage: (message) => {
				this.editor.taskErrorMessage = message;
				if (message) this.setPageErrorMessage(message);
			}
		});
	};

	currentWeek = () => this.summary?.week.code ?? '';
	taskWeek = () => this.summary?.currentWeek?.code ?? this.summary?.week.code ?? '';
	members = () => this.summary?.members ?? [];
	tasks = () => this.summary?.tasks ?? [];
	definitions = () => definitionsFromSummary(this.summary);
	statusOptions = (task?: Task | null) => statusOptionsFromSummary(this.summary, task);
	categoryOptions = () => categorySelectOptions(this.definitions(), this.text.report.etcLabel);
	typeOptions = () => typeSelectOptions(this.definitions(), this.text.report.etcLabel);
	sizeOptions = () => sizeSelectOptions(this.definitions());
	statusFilterOptions = () => this.filters.statusOptions(this.summary, this.text, this.statusLabel);
	memberFilterOptions = () => this.filters.memberOptions(this.summary, this.text);
	categoryFilterOptions = () => this.filters.businessOptions(this.summary, this.text);
	typeFilterOptions = () => this.filters.typeOptions(this.summary, this.text);
	statusSelectOptions = (task?: Task | null) => statusSelectOptions(this.statusOptions(task ?? this.taskDraft), this.statusLabel);
	businessColor = (business: string | null) => taskBusinessColor(business, this.definitions());
	taskTypeColor = (type: string | null) => taskTypeColor(type, this.definitions());
	participantScope = () => taskBoardParticipantScope(this.filters.participantFilterIDs, currentTaskMember(this.summary)?.id);
	memberEmail = (memberID: string) => this.members().find((member) => member.id === memberID)?.email ?? '';
	currentMemberID = () => currentTaskMember(this.summary)?.id ?? '';
	canUseTaskRelationships = () => this.summary?.source === 'supabase' || this.summary?.source === 'dev-mock';

	filteredTasks = () => this.filters.tasks(this.tasks());

	statusLabel = (status: string): string => taskStatusLabel(this.text, status);

	openTask = (task: Task): void => {
		if (isEmbeddedFrame()) {
			openTaskInNewWindow(task.id);
			return;
		}
		this.editor.openTask(task, this.isBoardTaskPending);
	};

	createTask = (status?: string): void => {
		this.editor.createTask(status, this.filters.participantFilterIDs);
	};

	createChildTask = (parentTaskID: string): void => {
		this.editor.createTask(undefined, [], parentTaskID);
	};

	setTaskParent = async (taskID: string, parentTaskID?: string): Promise<boolean> => {
		if (!this.canUseTaskRelationships()) return false;
		const updated = await this.relationships.setParent(taskID, parentTaskID);
		if (updated && this.editor.taskDraft?.id === taskID) {
			this.editor.taskDraft = { ...this.editor.taskDraft, parentTaskID };
		}
		return updated;
	};

	setTaskParents = async (taskIDs: string[], parentTaskID: string): Promise<boolean> => {
		if (!this.canUseTaskRelationships()) return false;
		return this.relationships.setParents(taskIDs, parentTaskID);
	};

	createQuickTask = (allowDuplicate = false): Promise<TaskQuickTaskCreateResult> => this.quickTask.createQuickTask(allowDuplicate);

	saveTask = this.editor.saveTask;

	updateTaskStatus = async (task: Task, nextStatus: string): Promise<void> => {
		this.pendingStatusTaskID = task.id;
		this.setPageErrorMessage('');
		const result = await updateTaskStatus({
			task,
			nextStatus,
			canUpdateTask: this.canUpdateTask,
			isTaskPending: this.isBoardTaskPending,
			loadTask: this.loadTask,
			weekCode: this.currentWeek(),
			saveErrorMessage: this.text.task.saveError
		});
		if (result.status === 'failed') this.setPageErrorMessage(result.errorMessage);
		this.pendingStatusTaskID = '';
	};

	moveTaskOnBoard = async (request: TaskBoardMoveRequest): Promise<void> => {
		await this.board.moveTaskOnBoard(request);
	};

	isBoardTaskPending = (taskID: string): boolean => this.board.isTaskPending(taskID);

	resetFilters = (): void => {
		this.filters.reset(this.summary);
	};

	setParticipantFilterIDs = (memberIDs: string[]): void => {
		this.filters.setParticipantIDs(memberIDs);
	};

	setParticipantIDs = this.editor.setParticipantIDs;
	removeParticipantID = this.editor.removeParticipantID;
	closeEditor = this.editor.closeEditor;

	canUpdateTask = (task: Task): boolean => canUpdateTask(this.summary, task);
	canDeleteTask = this.editor.canDeleteTask;
	canManageTaskAssignment = this.editor.canManageTaskAssignment;
	canRemoveParticipant = this.editor.canRemoveParticipant;
	deleteTask = this.editor.deleteTask;

}

function openTaskInNewWindow(taskID: string): void {
	const url = new URL(window.location.href);
	url.searchParams.set('task', taskID);
	openDetailWindow(url.toString());
}
