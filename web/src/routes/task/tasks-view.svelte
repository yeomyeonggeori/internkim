<script lang="ts">
	import TaskFilters from './task-filters.svelte';
	import TaskListView from './task-list-view.svelte';
	import TaskEditor from './task-editor.svelte';
	import TaskQuickAdd from './task-quick-add.svelte';
	import TaskWeekSelector from './task-week-selector.svelte';
	import TaskBoard from './task-board.svelte';
	import { taskBoardWeekPosition } from './task-board-week-position';
	import * as Tabs from '$lib/components/ui/tabs';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { createTasksController } from './tasks-controller.svelte';
	import type { LoadTask } from './task-load-tracker';
	import type { TaskSummary } from './task-types';
	import { taskText } from './text';

	type TaskPageText = typeof taskText.ko;

	type Props = {
		summary: TaskSummary | null;
		focusedTaskID: string;
		text: TaskPageText;
		isLoading: boolean;
		loadTask: LoadTask;
		selectWeek: (weekCode: string) => void;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, focusedTaskID, text, isLoading, loadTask, selectWeek, setPageErrorMessage }: Props = $props();

	const page = createTasksController();
	let taskViewTab = $state('board');
	$effect(() => {
		const nextSummary = summary;
		const nextText = text;
		const nextLoadTask = loadTask;
		const nextSetPageErrorMessage = setPageErrorMessage;
		untrack(() => {
			page.sync({
				summary: nextSummary,
				text: nextText,
				loadTask: nextLoadTask,
				setPageErrorMessage: nextSetPageErrorMessage,
				announceMove: (message) => toast.success(message)
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
	<Tabs.Root bind:value={taskViewTab} class="flex flex-col gap-4" data-task-results>
		<div class="flex flex-wrap items-center gap-2">
			<Tabs.List>
				<Tabs.Trigger value="board">{text.task.viewTabs.board}</Tabs.Trigger>
				<Tabs.Trigger value="list">{text.task.viewTabs.list}</Tabs.Trigger>
			</Tabs.List>
			<TaskWeekSelector
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
			<TaskFilters
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
			<TaskBoard
				memberEmail={page.memberEmail}
				tasks={page.filteredTasks()}
				allTasks={page.tasks()}
				boardText={text.task.board}
				etcLabel={text.task.etcLabel}
				statusLabel={page.statusLabel}
				openTask={page.openTask}
				createTask={page.createTask}
				moveTask={page.moveTaskOnBoard}
				pendingTaskIDs={page.board.pendingTaskIDs}
				canUpdateTask={page.canUpdateTask}
				weekStartISO={summary?.week.startISO ?? ''}
				weekEndISO={summary?.week.endISO ?? ''}
				participantScope={page.participantScope()}
				weekPosition={taskBoardWeekPosition(summary)}
				businessColor={page.businessColor}
				taskTypeColor={page.taskTypeColor}
				childProgressLabel={text.task.relationships.progressLabel}
			/>
		</Tabs.Content>
		<Tabs.Content value="list" class="min-h-[36rem]">
			<TaskListView
				memberEmail={page.memberEmail}
				businessColor={page.businessColor}
				taskTypeColor={page.taskTypeColor}
				tasks={page.filteredTasks()}
				{text}
				statusOptionsForTask={page.statusSelectOptions}
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
	<TaskQuickAdd
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

<TaskEditor
	businessColor={page.businessColor}
	taskTypeColor={page.taskTypeColor}
	tasks={page.tasks()}
	currentMemberID={page.currentMemberID()}
	canUseTaskRelationships={page.canUseTaskRelationships()}
	pendingRelationshipTaskIDs={page.relationships.pendingTaskIDs}
	memberEmail={page.memberEmail}
	bind:taskDraft={page.editor.taskDraft}
	isEditingTask={page.editor.isEditingTask}
	members={page.members()}
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
	setParticipantIDs={page.setParticipantIDs}
	removeParticipantID={page.removeParticipantID}
	canRemoveParticipant={page.canRemoveParticipant}
	saveTask={page.saveTask}
	deleteTask={page.deleteTask}
	canUpdateTask={page.canUpdateTask}
	canDeleteTask={page.canDeleteTask}
	canManageTaskAssignment={page.canManageTaskAssignment}
	isOwnTask={page.editor.isOwnTask}
	startEditingTask={page.editor.startEditingTask}
	closeEditor={page.closeEditor}
	openRelatedTask={page.openTask}
	setTaskParent={page.setTaskParent}
	setTaskParents={page.setTaskParents}
	createChildTask={page.createChildTask}
/>
