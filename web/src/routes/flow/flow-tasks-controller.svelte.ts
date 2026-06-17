import { createQuickFlowTask, deleteFlowTask, saveFlowTask } from './flow-api';
import { createFlowTaskBoardMove, type FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import { saveFlowTaskBoardMove } from './flow-task-board-save';
import { cloneFlowTask, createFlowTaskDraft, defaultFlowTaskOwner, participantSelectionFromNames } from './flow-task-draft';
import type { LoadFlow } from './flow-load-tracker';
import {
	buildBusinessFilterOptions,
	buildBusinessSelectOptions,
	buildMemberFilterOptions,
	canDeleteFlowTask,
	canManageFlowTaskAssignment,
	canRemoveFlowTaskParticipant,
	canUpdateFlowTask,
	defaultParticipantFilterIDs,
	filterFlowTasks
} from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowDefinitions, FlowMember, FlowSummary, FlowTask } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTasksControllerInput = {
	summary: FlowSummary | null;
	text: FlowPageText;
	loadFlow: LoadFlow;
	setPageErrorMessage: (message: string) => void;
};

const emptyDefinitions: FlowDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function createFlowTasksController() {
	return new FlowTasksController();
}

class FlowTasksController {
	summary = $state<FlowSummary | null>(null);
	text: FlowPageText = flowText.ko;
	taskDraft = $state<FlowTask | null>(null);
	searchText = $state('');
	statusFilter = $state('all');
	participantFilterIDs = $state<string[]>([]);
	businessFilter = $state('all');
	typeFilter = $state('all');
	taskErrorMessage = $state('');
	quickTaskDuplicateMessage = $state('');
	quickTaskDuplicatePrompt = $state('');
	quickTaskText = $state('');
	isCreatingQuickTask = $state(false);
	isSavingTask = $state(false);
	isDeletingTask = $state(false);
	pendingStatusTaskID = $state('');
	pendingBoardTaskIDs = $state<string[]>([]);
	private defaultParticipantFilterEmail = '';
	private hasCustomizedParticipantFilter = false;

	private loadFlow: LoadFlow;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadFlow = async () => false;
		this.setPageErrorMessage = () => {};
	}

	sync = (input: FlowTasksControllerInput): void => {
		this.summary = input.summary;
		this.text = input.text;
		this.loadFlow = input.loadFlow;
		this.setPageErrorMessage = input.setPageErrorMessage;
		this.syncDefaultParticipantFilter();
	};

	currentWeek = () => this.summary?.week.code ?? '';
	taskWeek = () => this.summary?.currentWeek?.code ?? this.summary?.week.code ?? '';
	members = () => this.summary?.members ?? [];
	tasks = () => this.summary?.tasks ?? [];
	definitions = () => this.summary?.definitions ?? emptyDefinitions;
	statusOptions = () => this.summary?.statusOptions ?? [];
	categoryOptions = () => buildBusinessSelectOptions(this.definitions(), this.text.report.fallbackBusiness);
	typeOptions = () => this.definitions().types.map((type) => ({ value: type, label: type }));
	sizeOptions = () => this.definitions().sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
	statusFilterOptions = () => [
		{ value: 'all', label: this.text.filters.all },
		...this.statusOptions().map((status) => ({ value: status, label: this.statusLabel(status) }))
	];
	memberFilterOptions = () => buildMemberFilterOptions(this.members(), this.text.filters.all);
	categoryFilterOptions = () => buildBusinessFilterOptions(this.definitions().categories, this.text.filters.all, this.text.report.fallbackBusiness);
	typeFilterOptions = () => [{ value: 'all', label: this.text.filters.all }, ...this.typeOptions()];
	statusSelectOptions = () => this.statusOptions().map((status) => ({ value: status, label: this.statusLabel(status) }));
	memberSelectOptions = () => this.members().map((member) => ({ value: member.id, label: member.name }));

	filteredTasks = () => {
		return filterFlowTasks(this.tasks(), {
			searchText: this.searchText,
			statusFilter: this.statusFilter,
			participantFilterIDs: this.participantFilterIDs,
			businessFilter: this.businessFilter,
			typeFilter: this.typeFilter
		});
	};

	clearStaleDuplicatePrompt = (): void => {
		if (!this.quickTaskDuplicateMessage || this.quickTaskText.trim() === this.quickTaskDuplicatePrompt) return;
		this.quickTaskDuplicateMessage = '';
		this.quickTaskDuplicatePrompt = '';
	};

	statusLabel = (status: string): string => {
		const labels = this.text.status as Record<string, string>;
		return labels[status] ?? status;
	};

	openTask = (task: FlowTask): void => {
		if (this.isBoardTaskPending(task.id)) return;
		this.taskDraft = cloneFlowTask(task);
		this.taskErrorMessage = '';
	};

	createTask = (status?: string): void => {
		const owner = this.defaultTaskOwner();
		if (!owner || !this.summary) return;
		this.taskDraft = createFlowTaskDraft(owner, this.definitions(), this.taskWeek());
		if (typeof status === 'string' && status) this.taskDraft.status = status;
		this.taskErrorMessage = '';
	};

	createQuickTask = async (allowDuplicate = false): Promise<void> => {
		const prompt = this.quickTaskText.trim();
		const owner = this.defaultTaskOwner();
		if (!prompt || !owner || !this.summary) return;
		this.isCreatingQuickTask = true;
		this.taskErrorMessage = '';
		if (!allowDuplicate) {
			this.quickTaskDuplicateMessage = '';
			this.quickTaskDuplicatePrompt = '';
		}
		try {
			const result = await createQuickFlowTask(
				{
					prompt,
					ownerID: owner.id,
					participantIDs: [owner.id],
					weekCode: this.taskWeek(),
					allowDuplicate
				},
				this.text.task.quickAddError
			);
			if (result.status === 'skipped_duplicate') {
				this.quickTaskDuplicateMessage = this.text.task.quickAddDuplicate;
				this.quickTaskDuplicatePrompt = prompt;
				return;
			}
			this.quickTaskText = '';
			this.quickTaskDuplicateMessage = '';
			this.quickTaskDuplicatePrompt = '';
			await this.loadFlow(this.currentWeek());
		} catch (error) {
			this.taskErrorMessage = error instanceof Error ? error.message : this.text.task.quickAddError;
		} finally {
			this.isCreatingQuickTask = false;
		}
	};

	saveTask = async (): Promise<void> => {
		if (!this.taskDraft || !this.canUpdateTask(this.taskDraft)) return;
		this.isSavingTask = true;
		this.taskErrorMessage = '';
		try {
			await saveFlowTask(this.taskDraft, this.text.task.saveError);
			this.taskDraft = null;
			await this.loadFlow(this.currentWeek());
		} catch (error) {
			this.taskErrorMessage = error instanceof Error ? error.message : this.text.task.saveError;
		} finally {
			this.isSavingTask = false;
		}
	};

	updateTaskStatus = async (task: FlowTask, nextStatus: string): Promise<void> => {
		if (!task.id || nextStatus === task.status) return;
		if (!this.canUpdateTask(task)) return;
		if (this.isBoardTaskPending(task.id)) return;
		this.pendingStatusTaskID = task.id;
		this.setPageErrorMessage('');
		try {
			await saveFlowTask({ ...task, status: nextStatus }, this.text.task.saveError);
			await this.loadFlow(this.currentWeek());
		} catch (error) {
			this.setPageErrorMessage(error instanceof Error ? error.message : this.text.task.saveError);
		} finally {
			this.pendingStatusTaskID = '';
		}
	};

	moveTaskOnBoard = async (request: FlowTaskBoardMoveRequest): Promise<void> => {
		if (!this.summary || this.pendingBoardTaskIDs.length > 0) return;
		const task = this.summary.tasks.find((candidate) => candidate.id === request.taskID);
		if (!task || !this.canUpdateTask(task)) return;
		const move = createFlowTaskBoardMove(this.summary.tasks, request);
		if (!move) return;

		const previousSummary = this.summary;
		const week = this.currentWeek();
		this.pendingBoardTaskIDs = move.updates.map((task) => task.id);
		this.setPageErrorMessage('');
		this.summary = { ...this.summary, tasks: move.tasks };

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
				this.summary = previousSummary;
			}
			if (saveResult === 'saved_with_reload_error' && this.currentWeek() === week && this.summary) {
				this.summary = { ...this.summary, tasks: move.tasks };
			}
		} finally {
			this.pendingBoardTaskIDs = [];
		}
	};

	isBoardTaskPending = (taskID: string): boolean => this.pendingBoardTaskIDs.includes(taskID);

	resetFilters = (): void => {
		this.searchText = '';
		this.statusFilter = 'all';
		this.participantFilterIDs = defaultParticipantFilterIDs(this.summary);
		this.hasCustomizedParticipantFilter = false;
		this.businessFilter = 'all';
		this.typeFilter = 'all';
	};

	setParticipantFilterIDs = (memberIDs: string[]): void => {
		this.participantFilterIDs = [...memberIDs];
		this.hasCustomizedParticipantFilter = true;
	};

	setTaskOwnerID = (memberID: string): void => {
		if (!this.taskDraft) return;
		const owner = this.members().find((member) => member.id === memberID);
		if (!owner) return;
		const memberByID = new Map(this.members().map((member) => [member.id, member]));
		const participantIDs = Array.from(new Set([owner.id, ...this.taskDraft.participantIDs]));
		const participants = participantIDs
			.map((participantID) => memberByID.get(participantID))
			.filter((member): member is FlowMember => Boolean(member));
		this.taskDraft.ownerID = owner.id;
		this.taskDraft.ownerName = owner.name;
		this.taskDraft.participantIDs = participants.map((participant) => participant.id);
		this.taskDraft.participantNames = participants.map((participant) => participant.name);
	};

	setParticipantNames = (names: string[]): void => {
		if (!this.taskDraft) return;
		const selection = participantSelectionFromNames(names, this.members(), this.taskDraft.ownerID);
		this.taskDraft.participantIDs = selection.participantIDs;
		this.taskDraft.participantNames = selection.participantNames;
	};

	removeParticipantID = (memberID: string): void => {
		if (!this.taskDraft || !canRemoveFlowTaskParticipant(this.taskDraft, memberID)) return;
		const nextParticipants = this.taskDraft.participantIDs
			.map((participantID, index) => ({
				id: participantID,
				name: this.taskDraft?.participantNames[index] ?? ''
			}))
			.filter((participant) => participant.id !== memberID);
		this.taskDraft.participantIDs = nextParticipants.map((participant) => participant.id);
		this.taskDraft.participantNames = nextParticipants.map((participant) => participant.name);
	};

	closeEditor = (): void => {
		this.taskDraft = null;
	};

	canUpdateTask = (task: FlowTask): boolean => canUpdateFlowTask(this.summary, task);
	canDeleteTask = (task: FlowTask): boolean => canDeleteFlowTask(this.summary, task);
	canManageTaskAssignment = (task: FlowTask): boolean => canManageFlowTaskAssignment(this.summary, task);

	deleteTask = async (task: FlowTask): Promise<void> => {
		if (!task.id || !this.canDeleteTask(task)) return;
		this.isDeletingTask = true;
		this.taskErrorMessage = '';
		try {
			await deleteFlowTask(task.id, this.text.task.deleteError);
			this.taskDraft = null;
			await this.loadFlow(this.currentWeek());
		} catch (error) {
			const message = error instanceof Error ? error.message : this.text.task.deleteError;
			this.taskErrorMessage = message;
			this.setPageErrorMessage(message);
		} finally {
			this.isDeletingTask = false;
		}
	};

	private defaultTaskOwner(): FlowMember | undefined {
		return defaultFlowTaskOwner(this.members(), this.summary?.currentUserEmail ?? '', '');
	}

	private syncDefaultParticipantFilter(): void {
		const currentEmail = this.summary?.currentUserEmail ?? '';
		if (!currentEmail || this.hasCustomizedParticipantFilter && this.defaultParticipantFilterEmail === currentEmail) return;
		if (this.defaultParticipantFilterEmail === currentEmail && this.participantFilterIDs.length > 0) return;
		this.participantFilterIDs = defaultParticipantFilterIDs(this.summary);
		this.defaultParticipantFilterEmail = currentEmail;
	}
}
