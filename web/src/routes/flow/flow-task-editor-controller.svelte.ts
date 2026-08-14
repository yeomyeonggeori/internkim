import {
	cloneFlowTask,
	createFlowTaskDraft,
	defaultFlowTaskOwner,
	removeFlowTaskParticipant,
	updateFlowTaskParticipantIDs
} from './flow-task-draft';
import { definitionsFromSummary } from './flow-task-options';
import { deleteFlowTaskDraft, saveFlowTaskDraft } from './flow-task-persistence';
import type { LoadFlow } from './flow-load-tracker';
import {
	canDeleteFlowTask,
	canManageFlowTaskAssignment,
	canRemoveFlowTaskParticipant,
	canUpdateFlowTask,
	currentFlowMember
} from './flow-task-workspace-model';
import { flowText } from './text';
import { isCentralFlowSource } from './flow-source';
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
	isEditingTask = $state(false);
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
		this.taskDraft = createFlowTaskDraft(owner, definitionsFromSummary(this.summary), this.taskWeek());
		this.isEditingTask = true;
		if (typeof status === 'string' && status) this.taskDraft.status = status;
		if (this.summary.source === 'supabase') {
			const isRequest = status === '요청';
			this.taskDraft.requesterID = isRequest ? owner.id : '';
			this.taskDraft.requesterName = isRequest ? owner.name : '';
			if (isRequest) {
				const targetIDs = targetParticipantIDs.length > 0 ? targetParticipantIDs : [owner.id];
				this.taskDraft = updateFlowTaskParticipantIDs(this.taskDraft, this.members(), targetIDs, this.summary.source);
			}
		}
		this.taskDraft.parentTaskID = parentTaskID;
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

	setParticipantIDs = (memberIDs: string[]): void => {
		if (!this.taskDraft) return;
		this.taskDraft = updateFlowTaskParticipantIDs(this.taskDraft, this.members(), memberIDs, this.summary?.source ?? '');
	};

	removeParticipantID = (memberID: string): void => {
		if (!this.taskDraft || !this.canRemoveParticipant(this.taskDraft, memberID)) return;
		this.taskDraft = removeFlowTaskParticipant(this.taskDraft, memberID, this.summary?.source ?? '');
	};

	closeEditor = (): void => {
		this.taskDraft = null;
		this.isEditingTask = false;
	};

	isOwnTask = (task: FlowTask): boolean => currentFlowMember(this.summary)?.id === task.ownerID;
	canUpdateTask = (task: FlowTask): boolean => canUpdateFlowTask(this.summary, task);
	canDeleteTask = (task: FlowTask): boolean => canDeleteFlowTask(this.summary, task);
	canManageTaskAssignment = (task: FlowTask): boolean => canManageFlowTaskAssignment(this.summary, task);
	canRemoveParticipant = (task: FlowTask, memberID: string): boolean => {
		if (!canRemoveFlowTaskParticipant(task, memberID)) return false;
		return isCentralFlowSource(this.summary?.source ?? '') || memberID !== task.ownerID;
	};

	private members(): FlowMember[] {
		return this.summary?.members ?? [];
	}

	private defaultTaskOwner(): FlowMember | undefined {
		return defaultFlowTaskOwner(this.members(), this.summary?.currentUserEmail ?? '', '');
	}
}
