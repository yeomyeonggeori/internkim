<script lang="ts">
	import FlowTaskFilters from './flow-task-filters.svelte';
	import FlowTaskListView from './flow-task-list-view.svelte';
	import FlowTaskEditor from './flow-task-editor.svelte';
	import FlowTaskQuickAdd from './flow-task-quick-add.svelte';
	import { createFlowTasksController } from './flow-tasks-controller.svelte';
	import type { FlowSummary } from './flow-types';
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

	const page = createFlowTasksController();

	$effect(() => {
		page.sync({ summary, activeMemberID, text, loadFlow, setPageErrorMessage });
	});

	$effect(page.clearStaleDuplicatePrompt);
</script>

<div class="space-y-4">
	<FlowTaskQuickAdd
		bind:quickTaskText={page.quickTaskText}
		taskErrorMessage={page.taskErrorMessage}
		quickTaskDuplicateMessage={page.quickTaskDuplicateMessage}
		isCreatingQuickTask={page.isCreatingQuickTask}
		hasMembers={page.members().length > 0}
		text={text.task}
		createQuickTask={() => page.createQuickTask(false)}
		confirmQuickTaskDuplicate={() => page.createQuickTask(true)}
	/>

	<FlowTaskFilters
		bind:searchText={page.searchText}
		bind:statusFilter={page.statusFilter}
		bind:ownerFilter={page.ownerFilter}
		bind:businessFilter={page.businessFilter}
		bind:typeFilter={page.typeFilter}
		statusOptions={page.statusFilterOptions()}
		ownerOptions={page.memberFilterOptions()}
		businessOptions={page.categoryFilterOptions()}
		typeOptions={page.typeFilterOptions()}
		hasBusinessFilter={page.definitions().categories.length > 1}
		hasMembers={page.members().length > 0}
		text={text.filters}
		resetFilters={page.resetFilters}
		createTask={page.createTask}
	/>

	<FlowTaskListView
		tasks={page.filteredTasks()}
		{text}
		statusOptions={page.statusSelectOptions()}
		pendingStatusTaskID={page.pendingStatusTaskID}
		statusLabel={page.statusLabel}
		updateTaskStatus={page.updateTaskStatus}
		openTask={page.openTask}
	/>
</div>

<FlowTaskEditor
	bind:taskDraft={page.taskDraft}
	members={page.members()}
	memberOptions={page.memberSelectOptions()}
	categoryOptions={page.categoryOptions()}
	typeOptions={page.typeOptions()}
	sizeOptions={page.sizeOptions()}
	statusOptions={page.statusSelectOptions()}
	taskErrorMessage={page.taskErrorMessage}
	isSavingTask={page.isSavingTask}
	pageTitle={text.title}
	text={text.task}
	statusLabel={page.statusLabel}
	setParticipantNames={page.setParticipantNames}
	saveTask={page.saveTask}
	closeEditor={page.closeEditor}
/>
