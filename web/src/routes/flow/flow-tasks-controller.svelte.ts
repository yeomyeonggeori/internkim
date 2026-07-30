import { isEmbeddedFrame, openDetailWindow } from '$lib/embedded';
import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import { FlowTaskBoardController } from './flow-task-board-controller.svelte';
import { FlowTaskEditorController } from './flow-task-editor-controller.svelte';
import { FlowTaskFiltersController } from './flow-task-filters-controller.svelte';
import { flowBoardParticipantScope } from './flow-board-participant-scope';
import { flowBusinessColor, flowTaskTypeColor } from './flow-definition-colors';
import { updateFlowTaskStatus } from './flow-task-persistence';
import { FlowTaskQuickCreateController } from './flow-task-quick-create-controller.svelte';
import type { LoadFlow } from './flow-load-tracker';
import {
	categorySelectOptions,
	definitionsFromSummary,
	flowTaskStatusLabel,
	memberSelectOptions,
	sizeSelectOptions,
	statusOptionsFromSummary,
	statusSelectOptions,
	typeSelectOptions
} from './flow-task-options';
import {
	canUpdateFlowTask,
	currentFlowMember,
} from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowQuickTaskCreateResult, FlowSummary, FlowTask } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTasksControllerInput = {
	summary: FlowSummary | null;
	text: FlowPageText;
	loadFlow: LoadFlow;
	setPageErrorMessage: (message: string) => void;
};

export function createFlowTasksController() {
	return new FlowTasksController();
}

class FlowTasksController {
	summary = $state<FlowSummary | null>(null);
	text: FlowPageText = flowText.ko;
	board = new FlowTaskBoardController();
	editor = new FlowTaskEditorController();
	filters = new FlowTaskFiltersController();
	quickTask = new FlowTaskQuickCreateController();
	pendingStatusTaskID = $state('');

	private loadFlow: LoadFlow;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadFlow = async () => false;
		this.setPageErrorMessage = () => {};
	}

	get taskDraft(): FlowTask | null {
		return this.editor.taskDraft;
	}

	set taskDraft(taskDraft: FlowTask | null) {
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

	sync = (input: FlowTasksControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadFlow = input.loadFlow;
		this.setPageErrorMessage = input.setPageErrorMessage;
		this.filters.sync(this.summary);
		this.quickTask.sync({
			summary: this.summary,
			text: this.text,
			loadFlow: this.loadFlow
		});
		this.board.sync({
			text: this.text,
			loadFlow: this.loadFlow,
			currentWeek: this.currentWeek,
			getSummary: () => this.summary,
			setSummary: (summary) => {
				this.summary = summary;
			},
			setPageErrorMessage: this.setPageErrorMessage
		});
		this.editor.sync({
			summary: this.summary,
			text: this.text,
			loadFlow: this.loadFlow,
			currentWeek: this.currentWeek,
			taskWeek: this.taskWeek,
			setPageErrorMessage: this.setPageErrorMessage
		});
	};

	currentWeek = () => this.summary?.week.code ?? '';
	taskWeek = () => this.summary?.currentWeek?.code ?? this.summary?.week.code ?? '';
	members = () => this.summary?.members ?? [];
	tasks = () => this.summary?.tasks ?? [];
	definitions = () => definitionsFromSummary(this.summary);
	statusOptions = () => statusOptionsFromSummary(this.summary);
	categoryOptions = () => categorySelectOptions(this.definitions(), this.text.report.fallbackBusiness);
	typeOptions = () => typeSelectOptions(this.definitions());
	sizeOptions = () => sizeSelectOptions(this.definitions());
	statusFilterOptions = () => this.filters.statusOptions(this.summary, this.text, this.statusLabel);
	memberFilterOptions = () => this.filters.memberOptions(this.summary, this.text);
	categoryFilterOptions = () => this.filters.businessOptions(this.summary, this.text);
	typeFilterOptions = () => this.filters.typeOptions(this.summary, this.text);
	statusSelectOptions = () => statusSelectOptions(this.statusOptions(), this.statusLabel);
	memberSelectOptions = () => memberSelectOptions(this.members());
	businessColor = (business: string) => flowBusinessColor(business, this.definitions());
	taskTypeColor = (type: string) => flowTaskTypeColor(type, this.definitions());
	participantScope = () => flowBoardParticipantScope(this.filters.participantFilterIDs, currentFlowMember(this.summary)?.id);
	memberEmail = (memberID: string) => this.members().find((member) => member.id === memberID)?.email ?? '';

	filteredTasks = () => this.filters.tasks(this.tasks());

	statusLabel = (status: string): string => flowTaskStatusLabel(this.text, status);

	openTask = (task: FlowTask): void => {
		if (isEmbeddedFrame()) {
			openTaskInNewWindow(task.id);
			return;
		}
		this.editor.openTask(task, this.isBoardTaskPending);
	};

	createTask = (status?: string): void => {
		this.editor.createTask(status);
	};

	createQuickTask = (allowDuplicate = false): Promise<FlowQuickTaskCreateResult> => this.quickTask.createQuickTask(allowDuplicate);

	saveTask = this.editor.saveTask;

	updateTaskStatus = async (task: FlowTask, nextStatus: string): Promise<void> => {
		this.pendingStatusTaskID = task.id;
		this.setPageErrorMessage('');
		const result = await updateFlowTaskStatus({
			task,
			nextStatus,
			canUpdateTask: this.canUpdateTask,
			isTaskPending: this.isBoardTaskPending,
			loadFlow: this.loadFlow,
			weekCode: this.currentWeek(),
			saveErrorMessage: this.text.task.saveError
		});
		if (result.status === 'failed') this.setPageErrorMessage(result.errorMessage);
		this.pendingStatusTaskID = '';
	};

	moveTaskOnBoard = async (request: FlowTaskBoardMoveRequest): Promise<void> => {
		await this.board.moveTaskOnBoard(request);
	};

	isBoardTaskPending = (taskID: string): boolean => this.board.isTaskPending(taskID);

	resetFilters = (): void => {
		this.filters.reset(this.summary);
	};

	setParticipantFilterIDs = (memberIDs: string[]): void => {
		this.filters.setParticipantIDs(memberIDs);
	};

	setTaskOwnerID = this.editor.setTaskOwnerID;
	setParticipantNames = this.editor.setParticipantNames;
	removeParticipantID = this.editor.removeParticipantID;
	closeEditor = this.editor.closeEditor;

	canUpdateTask = (task: FlowTask): boolean => canUpdateFlowTask(this.summary, task);
	canDeleteTask = this.editor.canDeleteTask;
	canManageTaskAssignment = this.editor.canManageTaskAssignment;
	deleteTask = this.editor.deleteTask;

}

function openTaskInNewWindow(taskID: string): void {
	const url = new URL(window.location.href);
	url.searchParams.set('task', taskID);
	openDetailWindow(url.toString());
}
