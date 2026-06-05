<script lang="ts">
	import { createQuickFlowTask, saveFlowTask } from './flow-api';
	import FlowTaskFilters from './flow-task-filters.svelte';
	import FlowTaskListView from './flow-task-list-view.svelte';
	import FlowTaskEditor from './flow-task-editor.svelte';
	import FlowTaskQuickAdd from './flow-task-quick-add.svelte';
	import { cloneFlowTask, createFlowTaskDraft, defaultFlowTaskOwner, participantSelectionFromNames } from './flow-task-draft';
	import type { FlowDefinitions, FlowMember, FlowSummary, FlowTask } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		summary: FlowSummary | null;
		activeMemberID: string;
		text: FlowPageText;
		loadFlow: (week: string) => Promise<void>;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, activeMemberID, text, loadFlow, setPageErrorMessage }: Props = $props();

	let taskDraft = $state<FlowTask | null>(null);
	let searchText = $state('');
	let statusFilter = $state('all');
	let ownerFilter = $state('all');
	let businessFilter = $state('all');
	let typeFilter = $state('all');
	let taskErrorMessage = $state('');
	let quickTaskDuplicateMessage = $state('');
	let quickTaskDuplicatePrompt = $state('');
	let quickTaskText = $state('');
	let isCreatingQuickTask = $state(false);
	let isSavingTask = $state(false);
	let pendingStatusTaskID = $state('');

	const emptyDefinitions: FlowDefinitions = {
		categories: [],
		types: [],
		sizes: []
	};

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const definitions = () => summary?.definitions ?? emptyDefinitions;
	const statusOptions = () => summary?.statusOptions ?? [];
	const categoryOptions = () => definitions().categories.map((category) => ({ value: category, label: category }));
	const typeOptions = () => definitions().types.map((type) => ({ value: type, label: type }));
	const sizeOptions = () =>
		definitions().sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
	const statusFilterOptions = () => [
		{ value: 'all', label: text.filters.all },
		...statusOptions().map((status) => ({ value: status, label: statusLabel(status) }))
	];
	const memberFilterOptions = () => [
		{ value: 'all', label: text.filters.all },
		...members().map((member) => ({ value: member.id, label: member.name }))
	];
	const categoryFilterOptions = () => [{ value: 'all', label: text.filters.all }, ...categoryOptions()];
	const typeFilterOptions = () => [{ value: 'all', label: text.filters.all }, ...typeOptions()];
	const statusSelectOptions = () => statusOptions().map((status) => ({ value: status, label: statusLabel(status) }));
	const memberSelectOptions = () => members().map((member) => ({ value: member.id, label: member.name }));

	const filteredTasks = $derived.by(() => {
		const normalizedSearch = searchText.trim().toLowerCase();
		return tasks().filter((task) => {
			if (statusFilter !== 'all' && task.status !== statusFilter) return false;
			if (ownerFilter !== 'all' && !task.participantIDs.includes(ownerFilter)) return false;
			if (businessFilter !== 'all' && task.business !== businessFilter) return false;
			if (typeFilter !== 'all' && task.type !== typeFilter) return false;
			if (activeMemberID && !task.participantIDs.includes(activeMemberID)) return false;
			if (!normalizedSearch) return true;
			return [task.content, task.goal, task.ownerName, task.business, task.type]
				.join(' ')
				.toLowerCase()
				.includes(normalizedSearch);
		});
	});

	$effect(() => {
		if (quickTaskDuplicateMessage && quickTaskText.trim() !== quickTaskDuplicatePrompt) {
			quickTaskDuplicateMessage = '';
			quickTaskDuplicatePrompt = '';
		}
	});

	function statusLabel(status: string): string {
		const labels = text.status as Record<string, string>;
		return labels[status] ?? status;
	}

	function openTask(task: FlowTask): void {
		taskDraft = cloneFlowTask(task);
		taskErrorMessage = '';
	}

	function createTask(): void {
		const owner = defaultTaskOwner();
		if (!owner || !summary) return;
		taskDraft = createFlowTaskDraft(owner, definitions(), summary.week.code);
		taskErrorMessage = '';
	}

	async function createQuickTask(allowDuplicate = false): Promise<void> {
		const prompt = quickTaskText.trim();
		const owner = defaultTaskOwner();
		if (!prompt || !owner || !summary) return;
		isCreatingQuickTask = true;
		taskErrorMessage = '';
		if (!allowDuplicate) {
			quickTaskDuplicateMessage = '';
			quickTaskDuplicatePrompt = '';
		}
		try {
			const result = await createQuickFlowTask(
				{
					prompt,
					ownerID: owner.id,
					participantIDs: [owner.id],
					weekCode: summary.week.code,
					allowDuplicate
				},
				text.task.quickAddError
			);
			if (result.status === 'skipped_duplicate') {
				quickTaskDuplicateMessage = text.task.quickAddDuplicate;
				quickTaskDuplicatePrompt = prompt;
				return;
			}
			quickTaskText = '';
			quickTaskDuplicateMessage = '';
			quickTaskDuplicatePrompt = '';
			await loadFlow(currentWeek());
		} catch (error) {
			taskErrorMessage = error instanceof Error ? error.message : text.task.quickAddError;
		} finally {
			isCreatingQuickTask = false;
		}
	}

	function defaultTaskOwner(): FlowMember | undefined {
		return defaultFlowTaskOwner(members(), summary?.currentUserEmail ?? '', activeMemberID);
	}

	async function saveTask(): Promise<void> {
		if (!taskDraft) return;
		isSavingTask = true;
		taskErrorMessage = '';
		try {
			await saveFlowTask(taskDraft, text.task.saveError);
			taskDraft = null;
			await loadFlow(currentWeek());
		} catch (error) {
			taskErrorMessage = error instanceof Error ? error.message : text.task.saveError;
		} finally {
			isSavingTask = false;
		}
	}

	async function updateTaskStatus(task: FlowTask, nextStatus: string): Promise<void> {
		if (!task.id || nextStatus === task.status) return;
		pendingStatusTaskID = task.id;
		setPageErrorMessage('');
		try {
			await saveFlowTask({ ...task, status: nextStatus }, text.task.saveError);
			await loadFlow(currentWeek());
		} catch (error) {
			setPageErrorMessage(error instanceof Error ? error.message : text.task.saveError);
		} finally {
			pendingStatusTaskID = '';
		}
	}

	function resetFilters(): void {
		searchText = '';
		statusFilter = 'all';
		ownerFilter = 'all';
		businessFilter = 'all';
		typeFilter = 'all';
	}

	function setParticipantNames(names: string[]): void {
		if (!taskDraft) return;
		const selection = participantSelectionFromNames(names, members(), taskDraft.ownerID);
		taskDraft.participantIDs = selection.participantIDs;
		taskDraft.participantNames = selection.participantNames;
	}

</script>

<div class="space-y-4">
	<FlowTaskQuickAdd
		bind:quickTaskText
		{taskErrorMessage}
		{quickTaskDuplicateMessage}
		{isCreatingQuickTask}
		hasMembers={members().length > 0}
		text={text.task}
		createQuickTask={() => createQuickTask(false)}
		confirmQuickTaskDuplicate={() => createQuickTask(true)}
	/>

	<FlowTaskFilters
		bind:searchText
		bind:statusFilter
		bind:ownerFilter
		bind:businessFilter
		bind:typeFilter
		statusOptions={statusFilterOptions()}
		ownerOptions={memberFilterOptions()}
		businessOptions={categoryFilterOptions()}
		typeOptions={typeFilterOptions()}
		hasBusinessFilter={definitions().categories.length > 1}
		hasMembers={members().length > 0}
		text={text.filters}
		{resetFilters}
		{createTask}
	/>

	<FlowTaskListView
		tasks={filteredTasks}
		{text}
		statusOptions={statusSelectOptions()}
		{pendingStatusTaskID}
		{statusLabel}
		{updateTaskStatus}
		{openTask}
	/>
</div>

<FlowTaskEditor
	bind:taskDraft
	members={members()}
	memberOptions={memberSelectOptions()}
	categoryOptions={categoryOptions()}
	typeOptions={typeOptions()}
	sizeOptions={sizeOptions()}
	statusOptions={statusSelectOptions()}
	{taskErrorMessage}
	{isSavingTask}
	pageTitle={text.title}
	text={text.task}
	{statusLabel}
	{setParticipantNames}
	{saveTask}
	closeEditor={() => {
		taskDraft = null;
	}}
/>
