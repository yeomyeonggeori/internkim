import { createQuickFlowTask, saveFlowTask } from './flow-api';
import { cloneFlowTask, createFlowTaskDraft, defaultFlowTaskOwner, participantSelectionFromNames } from './flow-task-draft';
import { flowText } from './text';
import type { FlowDefinitions, FlowMember, FlowSummary, FlowTask } from './flow-types';

type FlowPageText = typeof flowText.ko;

type FlowTasksControllerInput = {
	summary: FlowSummary | null;
	activeMemberID: string;
	text: FlowPageText;
	loadFlow: (week: string) => Promise<void>;
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
	activeMemberID = $state('');
	text: FlowPageText = flowText.ko;
	taskDraft = $state<FlowTask | null>(null);
	searchText = $state('');
	statusFilter = $state('all');
	ownerFilter = $state('all');
	businessFilter = $state('all');
	typeFilter = $state('all');
	taskErrorMessage = $state('');
	quickTaskDuplicateMessage = $state('');
	quickTaskDuplicatePrompt = $state('');
	quickTaskText = $state('');
	isCreatingQuickTask = $state(false);
	isSavingTask = $state(false);
	pendingStatusTaskID = $state('');

	private loadFlow: (week: string) => Promise<void>;
	private setPageErrorMessage: (message: string) => void;

	constructor() {
		this.loadFlow = async () => {};
		this.setPageErrorMessage = () => {};
	}

	sync = (input: FlowTasksControllerInput): void => {
		this.summary = input.summary;
		this.activeMemberID = input.activeMemberID;
		this.text = input.text;
		this.loadFlow = input.loadFlow;
		this.setPageErrorMessage = input.setPageErrorMessage;
	};

	currentWeek = () => this.summary?.week.code ?? '';
	taskWeek = () => this.summary?.currentWeek?.code ?? this.summary?.week.code ?? '';
	members = () => this.summary?.members ?? [];
	tasks = () => this.summary?.tasks ?? [];
	definitions = () => this.summary?.definitions ?? emptyDefinitions;
	statusOptions = () => this.summary?.statusOptions ?? [];
	categoryOptions = () => this.definitions().categories.map((category) => ({ value: category, label: category }));
	typeOptions = () => this.definitions().types.map((type) => ({ value: type, label: type }));
	sizeOptions = () => this.definitions().sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
	statusFilterOptions = () => [
		{ value: 'all', label: this.text.filters.all },
		...this.statusOptions().map((status) => ({ value: status, label: this.statusLabel(status) }))
	];
	memberFilterOptions = () => [
		{ value: 'all', label: this.text.filters.all },
		...this.members().map((member) => ({ value: member.id, label: member.name }))
	];
	categoryFilterOptions = () => [{ value: 'all', label: this.text.filters.all }, ...this.categoryOptions()];
	typeFilterOptions = () => [{ value: 'all', label: this.text.filters.all }, ...this.typeOptions()];
	statusSelectOptions = () => this.statusOptions().map((status) => ({ value: status, label: this.statusLabel(status) }));
	memberSelectOptions = () => this.members().map((member) => ({ value: member.id, label: member.name }));
	personalTasks = () => {
		return this.tasks().filter((task) => !this.activeMemberID || task.participantIDs.includes(this.activeMemberID));
	};

	filteredTasks = () => {
		const normalizedSearch = this.searchText.trim().toLowerCase();
		return this.personalTasks().filter((task) => {
			if (this.statusFilter !== 'all' && task.status !== this.statusFilter) return false;
			if (this.ownerFilter !== 'all' && !task.participantIDs.includes(this.ownerFilter)) return false;
			if (this.businessFilter !== 'all' && task.business !== this.businessFilter) return false;
			if (this.typeFilter !== 'all' && task.type !== this.typeFilter) return false;
			if (!normalizedSearch) return true;
			return [task.content, task.goal, task.ownerName, task.business, task.type].join(' ').toLowerCase().includes(normalizedSearch);
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
		if (!this.taskDraft) return;
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

	resetFilters = (): void => {
		this.searchText = '';
		this.statusFilter = 'all';
		this.ownerFilter = 'all';
		this.businessFilter = 'all';
		this.typeFilter = 'all';
	};

	setParticipantNames = (names: string[]): void => {
		if (!this.taskDraft) return;
		const selection = participantSelectionFromNames(names, this.members(), this.taskDraft.ownerID);
		this.taskDraft.participantIDs = selection.participantIDs;
		this.taskDraft.participantNames = selection.participantNames;
	};

	closeEditor = (): void => {
		this.taskDraft = null;
	};

	private defaultTaskOwner(): FlowMember | undefined {
		return defaultFlowTaskOwner(this.members(), this.summary?.currentUserEmail ?? '', this.activeMemberID);
	}
}
