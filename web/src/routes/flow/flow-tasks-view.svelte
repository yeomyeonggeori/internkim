<script lang="ts">
	import FlowTaskFilters from './flow-task-filters.svelte';
	import FlowTaskListView from './flow-task-list-view.svelte';
	import FlowTaskEditor from './flow-task-editor.svelte';
	import FlowTaskQuickAdd from './flow-task-quick-add.svelte';
	import FlowTaskBoard from './flow-task-board.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createFlowTasksController } from './flow-tasks-controller.svelte';
	import type { FlowSummary } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		summary: FlowSummary | null;
		activeMemberID: string;
		focusedTaskID: string;
		text: FlowPageText;
		loadFlow: (week: string) => Promise<void>;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, activeMemberID, focusedTaskID, text, loadFlow, setPageErrorMessage }: Props = $props();

	const page = createFlowTasksController();
	let taskViewTab = $state('list');
	const taskViewTabTriggerClass = [
		'h-8 min-w-16 flex-none rounded-full px-4 after:hidden',
		'aria-selected:bg-primary aria-selected:font-semibold',
		'aria-selected:text-primary-foreground',
		'data-active:bg-primary data-active:font-semibold data-active:text-primary-foreground'
	].join(' ');

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

	<Tabs.Root bind:value={taskViewTab} class="space-y-4">
		<Tabs.List class="h-10 rounded-full border bg-muted/50 p-1">
			<Tabs.Trigger value="board" class={taskViewTabTriggerClass}>
				{text.task.viewTabs.board}
			</Tabs.Trigger>
			<Tabs.Trigger value="list" class={taskViewTabTriggerClass}>
				{text.task.viewTabs.list}
			</Tabs.Trigger>
		</Tabs.List>
		<Tabs.Content value="board">
			<FlowTaskBoard
				tasks={page.filteredTasks()}
				boardText={text.task.board}
				statusLabel={page.statusLabel}
				openTask={page.openTask}
				createTask={page.createTask}
			/>
		</Tabs.Content>
		<Tabs.Content value="list">
			<FlowTaskListView
				tasks={page.filteredTasks()}
				{text}
				statusOptions={page.statusSelectOptions()}
				pendingStatusTaskID={page.pendingStatusTaskID}
				statusLabel={page.statusLabel}
				updateTaskStatus={page.updateTaskStatus}
				openTask={page.openTask}
				{focusedTaskID}
			/>
		</Tabs.Content>
	</Tabs.Root>
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
