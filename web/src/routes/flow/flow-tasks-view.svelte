<script lang="ts">
	import FlowTaskFilters from './flow-task-filters.svelte';
	import FlowTaskListView from './flow-task-list-view.svelte';
	import FlowTaskEditor from './flow-task-editor.svelte';
	import FlowTaskQuickAdd from './flow-task-quick-add.svelte';
	import FlowWeekSelector from './flow-week-selector.svelte';
	import FlowTaskBoard from './flow-task-board.svelte';
	import { flowTaskBoardWeekPosition } from './flow-task-board-week-position';
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
		isLoading: boolean;
		loadFlow: LoadFlow;
		selectWeek: (weekCode: string) => void;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, focusedTaskID, text, isLoading, loadFlow, selectWeek, setPageErrorMessage }: Props = $props();

	const page = createFlowTasksController();
	let taskViewTab = $state('board');
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

	$effect(page.quickTask.clearStaleDuplicatePrompt);

	let openedFocusedTaskID = '';

	$effect(() => {
		if (!focusedTaskID || focusedTaskID === openedFocusedTaskID) return;
		const focusedTask = page.tasks().find((task) => task.id === focusedTaskID);
		if (!focusedTask) return;
		openedFocusedTaskID = focusedTaskID;
		untrack(() => page.openTask(focusedTask));
	});
</script>

<div class={taskViewTab === 'board' ? 'flex flex-col gap-4 pb-0' : 'flex flex-col gap-4 pb-36 md:pb-16'}>
	<Tabs.Root bind:value={taskViewTab} class="flex flex-col gap-4" data-flow-task-results>
		<div class="flex flex-wrap items-center gap-2">
			<Tabs.List>
				<Tabs.Trigger value="board">{text.task.viewTabs.board}</Tabs.Trigger>
				<Tabs.Trigger value="list">{text.task.viewTabs.list}</Tabs.Trigger>
			</Tabs.List>
			<FlowWeekSelector
				class="ml-auto"
				week={summary?.week}
				currentWeekStartISO={summary?.currentWeek?.startISO ?? ''}
				disabled={!summary || isLoading}
				selectWeekLabel={text.selectWeekDate}
				currentWeekLabel={text.currentWeek}
				lastWeekLabel={text.lastWeek}
				previousWeekLabel={text.previousWeek}
				nextWeekLabel={text.nextWeek}
				onSelectWeek={selectWeek}
			/>
			<FlowTaskFilters
				class="justify-end"
				bind:searchText={page.filters.searchText}
				bind:statusFilter={page.filters.statusFilter}
				bind:businessFilter={page.filters.businessFilter}
				bind:typeFilter={page.filters.typeFilter}
				participantFilterIDs={page.filters.participantFilterIDs}
				statusOptions={page.statusFilterOptions()}
				participantOptions={page.memberFilterOptions()}
				businessOptions={page.categoryFilterOptions()}
				typeOptions={page.typeFilterOptions()}
				hasBusinessFilter={page.definitions().categories.length > 0}
				text={text.filters}
				resetFilters={page.resetFilters}
				setParticipantFilterIDs={page.setParticipantFilterIDs}
			/>
		</div>
		<Tabs.Content value="board" class="min-h-[36rem]">
			<FlowTaskBoard
				memberEmail={page.memberEmail}
				tasks={page.filteredTasks()}
				boardText={text.task.board}
				businessFallback={text.task.businessFallback}
				statusLabel={page.statusLabel}
				openTask={page.openTask}
				createTask={page.createTask}
				moveTask={page.moveTaskOnBoard}
				pendingTaskIDs={page.board.pendingTaskIDs}
				canUpdateTask={page.canUpdateTask}
				weekStartISO={summary?.week.startISO ?? ''}
				weekEndISO={summary?.week.endISO ?? ''}
				weekPosition={flowTaskBoardWeekPosition(summary)}
			/>
		</Tabs.Content>
		<Tabs.Content value="list" class="min-h-[36rem]">
			<FlowTaskListView
				memberEmail={page.memberEmail}
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

{#if page.editor.taskDraft === null}
	<FlowTaskQuickAdd
		bind:quickTaskText={page.quickTask.quickTaskText}
		taskErrorMessage={page.quickTask.taskErrorMessage}
		quickTaskDuplicateMessage={page.quickTask.quickTaskDuplicateMessage}
		isCreatingQuickTask={page.quickTask.isCreatingQuickTask}
		hasMembers={page.members().length > 0}
		text={text.task}
		createQuickTask={() => page.createQuickTask(false)}
		confirmQuickTaskDuplicate={() => page.createQuickTask(true)}
	/>
{/if}

<FlowTaskEditor
	bind:taskDraft={page.editor.taskDraft}
	isEditingTask={page.editor.isEditingTask}
	members={page.members()}
	memberOptions={page.memberSelectOptions()}
	categoryOptions={page.categoryOptions()}
	typeOptions={page.typeOptions()}
	sizeOptions={page.sizeOptions()}
	statusOptions={page.statusSelectOptions()}
	taskErrorMessage={page.editor.taskErrorMessage}
	isSavingTask={page.editor.isSavingTask}
	isDeletingTask={page.editor.isDeletingTask}
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
	isOwnTask={page.editor.isOwnTask}
	startEditingTask={page.editor.startEditingTask}
	closeEditor={page.closeEditor}
/>
