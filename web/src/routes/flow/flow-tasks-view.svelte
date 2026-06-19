<script lang="ts">
	import FlowTaskFilters from './flow-task-filters.svelte';
	import FlowTaskListView from './flow-task-list-view.svelte';
	import FlowTaskEditor from './flow-task-editor.svelte';
	import FlowTaskQuickAdd from './flow-task-quick-add.svelte';
	import FlowTaskBoard from './flow-task-board.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { untrack } from 'svelte';
	import { createFlowTasksController } from './flow-tasks-controller.svelte';
	import type { LoadFlow } from './flow-load-tracker';
	import type { FlowSummary } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		summary: FlowSummary | null;
		focusedTaskID: string;
		text: FlowPageText;
		loadFlow: LoadFlow;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, focusedTaskID, text, loadFlow, setPageErrorMessage }: Props = $props();

	const page = createFlowTasksController();
	let taskViewTab = $state('board');
	const taskViewTabTriggerClass = [
		'h-8 min-w-16 flex-none rounded-full px-4 after:hidden',
		'aria-selected:bg-primary aria-selected:font-semibold',
		'aria-selected:text-primary-foreground',
		'data-active:bg-primary data-active:font-semibold data-active:text-primary-foreground'
	].join(' ');

	$effect(() => {
		const nextSummary = summary;
		const nextText = text;
		const nextLoadFlow = loadFlow;
		const nextSetPageErrorMessage = setPageErrorMessage;
		untrack(() => {
			page.sync({
				summary: nextSummary,
				text: nextText,
				loadFlow: nextLoadFlow,
				setPageErrorMessage: nextSetPageErrorMessage
			});
		});
	});

	$effect(page.clearStaleDuplicatePrompt);
</script>

<div class="space-y-4 pb-20 md:pb-16">
	<Tabs.Root bind:value={taskViewTab} class="space-y-4" data-flow-task-results>
		<div class="flex flex-wrap items-center gap-2">
			<Tabs.List class="h-10 rounded-full border bg-muted/50 p-1">
				<Tabs.Trigger value="board" class={taskViewTabTriggerClass}>
					{text.task.viewTabs.board}
				</Tabs.Trigger>
				<Tabs.Trigger value="list" class={taskViewTabTriggerClass}>
					{text.task.viewTabs.list}
				</Tabs.Trigger>
			</Tabs.List>
			<FlowTaskFilters
				bind:searchText={page.searchText}
				bind:statusFilter={page.statusFilter}
				bind:businessFilter={page.businessFilter}
				bind:typeFilter={page.typeFilter}
				participantFilterIDs={page.participantFilterIDs}
				statusOptions={page.statusFilterOptions()}
				participantOptions={page.memberFilterOptions()}
				businessOptions={page.categoryFilterOptions()}
				typeOptions={page.typeFilterOptions()}
				hasBusinessFilter={page.definitions().categories.length > 0}
				hasMembers={page.members().length > 0}
				text={text.filters}
				resetFilters={page.resetFilters}
				createTask={page.createTask}
				setParticipantFilterIDs={page.setParticipantFilterIDs}
			/>
		</div>
		<Tabs.Content value="board" class="min-h-[36rem]">
			<FlowTaskBoard
				tasks={page.filteredTasks()}
				boardText={text.task.board}
				businessFallback={text.task.businessFallback}
				statusLabel={page.statusLabel}
				openTask={page.openTask}
				createTask={page.createTask}
				moveTask={page.moveTaskOnBoard}
				pendingTaskIDs={page.pendingBoardTaskIDs}
				canUpdateTask={page.canUpdateTask}
				weekStartISO={summary?.week.startISO ?? ''}
				weekEndISO={summary?.week.endISO ?? ''}
			/>
		</Tabs.Content>
		<Tabs.Content value="list" class="min-h-[36rem]">
			<FlowTaskListView
				tasks={page.filteredTasks()}
				{text}
				statusOptions={page.statusSelectOptions()}
				pendingStatusTaskID={page.pendingStatusTaskID}
				statusLabel={page.statusLabel}
				updateTaskStatus={page.updateTaskStatus}
				openTask={page.openTask}
				canUpdateTask={page.canUpdateTask}
				{focusedTaskID}
			/>
		</Tabs.Content>
	</Tabs.Root>
</div>

{#if page.taskDraft === null}
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
{/if}

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
	isDeletingTask={page.isDeletingTask}
	pageTitle={text.title}
	text={text.task}
	statusLabel={page.statusLabel}
	setTaskOwnerID={page.setTaskOwnerID}
	setParticipantNames={page.setParticipantNames}
	removeParticipantID={page.removeParticipantID}
	saveTask={page.saveTask}
	deleteTask={page.deleteTask}
	canUpdateTask={page.canUpdateTask}
	canDeleteTask={page.canDeleteTask}
	canManageTaskAssignment={page.canManageTaskAssignment}
	closeEditor={page.closeEditor}
/>
