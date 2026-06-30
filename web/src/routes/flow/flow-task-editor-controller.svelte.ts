import {
	cloneFlowTask,
	createFlowTaskDraft,
	defaultFlowTaskOwner,
	removeFlowTaskParticipant,
	updateFlowTaskOwner,
	updateFlowTaskParticipantNames
} from './flow-task-draft';
import { definitionsFromSummary } from './flow-task-options';
import { deleteFlowTaskDraft, saveFlowTaskDraft } from './flow-task-persistence';
import type { LoadFlow } from './flow-load-tracker';
import {
	canDeleteFlowTask,
	canManageFlowTaskAssignment,
	canRemoveFlowTaskParticipant,
	canUpdateFlowTask
} from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowMember, FlowSummary, FlowTask } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTaskEditorControllerInput = {
	summary: FlowSummary | null;
	text: FlowPageText;
	loadFlow: LoadFlow;
	currentWeek: () => string;
	taskWeek: () => string;
	setPageErrorMessage: (message: string) => void;
};

export class FlowTaskEditorController {
	taskDraft = $state<FlowTask | null>(null);
	taskErrorMessage = $state('');
	isSavingTask = $state(false);
	isDeletingTask = $state(false);

	private summary = $state<FlowSummary | null>(null);
	private text: FlowPageText = flowText.ko;
	private loadFlow: LoadFlow;
	private currentWeek: () => string;
	private taskWeek: () => string;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadFlow = async () => false;
		this.currentWeek = () => '';
		this.taskWeek = () => '';
		this.setPageErrorMessage = () => {};
	}

	sync = (input: FlowTaskEditorControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadFlow = input.loadFlow;
		this.currentWeek = input.currentWeek;
		this.taskWeek = input.taskWeek;
		this.setPageErrorMessage = input.setPageErrorMessage;
	};

	openTask = (task: FlowTask, isTaskPending: (taskID: string) => boolean): void => {
		if (isTaskPending(task.id)) return;
		this.taskDraft = cloneFlowTask(task);
		this.taskErrorMessage = '';
	};

	createTask = (status?: string): void => {
		const owner = this.defaultTaskOwner();
		if (!owner || !this.summary) return;
		this.taskDraft = createFlowTaskDraft(owner, definitionsFromSummary(this.summary), this.taskWeek());
		if (typeof status === 'string' && status) this.taskDraft.status = status;
		this.taskErrorMessage = '';
	};

	saveTask = async (): Promise<void> => {
		this.isSavingTask = true;
		this.taskErrorMessage = '';
		const result = await saveFlowTaskDraft({
			task: this.taskDraft,
			canUpdateTask: this.canUpdateTask,
			loadFlow: this.loadFlow,
			weekCode: this.currentWeek(),
			saveErrorMessage: this.text.task.saveError
		});
		if (result.status === 'saved') this.taskDraft = null;
		if (result.status === 'failed') this.taskErrorMessage = result.errorMessage;
		this.isSavingTask = false;
	};

	deleteTask = async (task: FlowTask): Promise<void> => {
		this.isDeletingTask = true;
		this.taskErrorMessage = '';
		const result = await deleteFlowTaskDraft({
			task,
			canDeleteTask: this.canDeleteTask,
			loadFlow: this.loadFlow,
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

	setTaskOwnerID = (memberID: string): void => {
		if (!this.taskDraft) return;
		this.taskDraft = updateFlowTaskOwner(this.taskDraft, this.members(), memberID);
	};

	setParticipantNames = (names: string[]): void => {
		if (!this.taskDraft) return;
		this.taskDraft = updateFlowTaskParticipantNames(this.taskDraft, this.members(), names);
	};

	removeParticipantID = (memberID: string): void => {
		if (!this.taskDraft || !canRemoveFlowTaskParticipant(this.taskDraft, memberID)) return;
		this.taskDraft = removeFlowTaskParticipant(this.taskDraft, memberID);
	};

	closeEditor = (): void => {
		this.taskDraft = null;
	};

	canUpdateTask = (task: FlowTask): boolean => canUpdateFlowTask(this.summary, task);
	canDeleteTask = (task: FlowTask): boolean => canDeleteFlowTask(this.summary, task);
	canManageTaskAssignment = (task: FlowTask): boolean => canManageFlowTaskAssignment(this.summary, task);

	private members(): FlowMember[] {
		return this.summary?.members ?? [];
	}

	private defaultTaskOwner(): FlowMember | undefined {
		return defaultFlowTaskOwner(this.members(), this.summary?.currentUserEmail ?? '', '');
	}
}
