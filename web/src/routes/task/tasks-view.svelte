<script lang="ts">
	import TaskFilters from './task-filters.svelte';
	import TaskListView from './task-list-view.svelte';
	import TaskEditor from './task-editor.svelte';
	import TaskQuickAdd from './task-quick-add.svelte';
	import TaskWeekSelector from './task-week-selector.svelte';
	import TaskMonthSelector from './task-month-selector.svelte';
	import TaskRoadmap from './task-roadmap.svelte';
	import TaskRoadmapSkeleton from './task-roadmap-skeleton.svelte';
	import { localDateISO, taskRoadmapMonthOf } from './task-roadmap-model';
	import TaskBoard from './task-board.svelte';
	import TaskBoardSkeleton from './task-board-skeleton.svelte';
	import TaskChildCompletionDialog from './task-child-completion-dialog.svelte';
	import TaskContentSkeleton from './task-content-skeleton.svelte';
	import { taskBoardWeekPosition } from './task-board-week-position';
	import * as Tabs from '$lib/components/ui/tabs';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { createTasksController } from './tasks-controller.svelte';
	import type { LoadTask } from './task-load-tracker';
	import { relationshipTasksWithSnapshot } from './task-relationship-snapshot';
	import type { Task, TaskSummary } from './task-types';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		summary: TaskSummary | null;
		shownWeek: TaskSummary['week'] | undefined;
		canNavigateWeek: boolean;
		focusedTaskID: string;
		text: TaskPageText;
		isLoading: boolean;
		isPendingRead: boolean;
		loadError: string;
		ensureFullState: () => Promise<boolean>;
		relationshipSnapshotTasks: Task[] | null;
		isHistoryLoading: boolean;
		historyError: string;
		loadTask: LoadTask;
		selectWeek: (weekCode: string) => void;
		setPageErrorMessage: (message: string) => void;
	};

	let { summary, shownWeek, canNavigateWeek, focusedTaskID, text, isLoading, isPendingRead, loadError, ensureFullState, relationshipSnapshotTasks, isHistoryLoading, historyError, loadTask, selectWeek, setPageErrorMessage }: Props = $props();
	const hasShownWeek = $derived(Boolean(summary && summary.week.code === shownWeek?.code));

	const page = createTasksController();
	let relationshipTasks = $derived(summary?.completeness !== 'full' && relationshipSnapshotTasks ? relationshipTasksWithSnapshot(page.tasks(), relationshipSnapshotTasks) : page.tasks());
	let taskViewTab = $state('board');
	const todayISO = localDateISO(new Date());
	const currentRoadmapMonth = taskRoadmapMonthOf(todayISO);
	let roadmapMonth = $state(currentRoadmapMonth);
	$effect(() => {
		if (taskViewTab === 'list' || taskViewTab === 'roadmap') untrack(() => { void ensureFullState(); });
	});
	$effect(() => {
		const nextSummary = summary;
		const nextText = text;
		const nextLoadTask = loadTask;
		const nextSetPageErrorMessage = setPageErrorMessage;
		const relationshipsReady = nextSummary?.completeness === 'full' && !isLoading && !isHistoryLoading;
		untrack(() => {
			page.sync({
				summary: nextSummary,
				relationshipsReady,
				text: nextText,
				loadTask: nextLoadTask,
				setPageErrorMessage: nextSetPageErrorMessage,
				announceMove: (message) => toast.success(message),
				announceMoveFailure: (message) => toast.error(message)
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

<div class={taskViewTab === 'board' ? 'flex flex-col gap-4 pb-0 max-sm:pb-24' : 'flex flex-col gap-4 pb-36 md:pb-16'}>
	<Tabs.Root bind:value={taskViewTab} class="flex flex-col gap-4" data-task-results>
		<div class="flex flex-wrap items-center gap-2 max-sm:gap-1.5">
			<Tabs.List>
				<Tabs.Trigger value="board">{text.task.viewTabs.board}</Tabs.Trigger>
				<Tabs.Trigger value="list" disabled={!summary}>{text.task.viewTabs.list}</Tabs.Trigger>
				<Tabs.Trigger value="roadmap" disabled={!summary}>{text.task.viewTabs.roadmap}</Tabs.Trigger>
			</Tabs.List>
			{#if taskViewTab === 'roadmap'}
			<TaskMonthSelector
				class="ml-auto"
				month={roadmapMonth}
				currentMonth={currentRoadmapMonth}
				locale={text.task.roadmap.dateLocale}
				selectMonthLabel={text.task.roadmap.selectMonth}
				currentMonthLabel={text.task.roadmap.currentMonth}
				previousMonthLabel={text.task.roadmap.previousMonth}
				nextMonthLabel={text.task.roadmap.nextMonth}
				onSelectMonth={(month) => { roadmapMonth = month; }}
			/>
			{:else}
			<TaskWeekSelector
				class="ml-auto"
				week={shownWeek}
				currentWeekStartISO={summary?.currentWeek?.startISO ?? ''}
				disabled={!canNavigateWeek}
				selectWeekLabel={text.selectWeekDate}
				currentWeekLabel={text.currentWeek}
				lastWeekLabel={text.lastWeek}
				previousWeekLabel={text.previousWeek}
				nextWeekLabel={text.nextWeek}
				onSelectWeek={selectWeek}
			/>
			{/if}
			<TaskFilters
				class="justify-end"
				bind:searchText={page.filters.searchText}
				bind:statusFilter={page.filters.statusFilter}
				bind:businessFilter={page.filters.businessFilter}
				bind:typeFilter={page.filters.typeFilter}
				participantFilterIDs={page.filters.participantFilterIDs}
				statusOptions={page.statusFilterOptions()}
				participantOptions={page.memberFilterOptions()}
				peopleReady={summary?.peopleReady ?? false}
				businessOptions={page.categoryFilterOptions()}
				typeOptions={page.typeFilterOptions()}
				hasBusinessFilter={page.definitions().categories.length > 0}
				text={text.filters}
				resetFilters={page.resetFilters}
				setParticipantFilterIDs={page.setParticipantFilterIDs}
			/>
		</div>
		<Tabs.Content value="board" class="min-h-[36rem] max-sm:min-h-0">
			{#if !hasShownWeek}
				{#if isPendingRead && !loadError}<TaskBoardSkeleton label={text.loading} statusLabel={page.statusLabel} />{/if}
			{:else}
			<TaskBoard
				memberEmail={page.memberEmail}
				tasks={page.filteredTasks()}
				allTasks={page.tasks()}
				serverChildProgress={summary?.childProgressByParent}
				boardText={text.task.board}
				etcLabel={text.task.etcLabel}
				statusLabel={page.statusLabel}
				openTask={page.openTask}
				createTask={(status) => { if (!isLoading) page.createTask(status); }}
				moveTask={page.moveTaskOnBoard}
				pendingTaskIDs={page.board.pendingTaskIDs}
				canUpdateTask={isLoading ? () => false : page.canUpdateTask}
				weekStartISO={summary?.week.startISO ?? ''}
				weekEndISO={summary?.week.endISO ?? ''}
				participantScope={page.participantScope()}
				weekPosition={taskBoardWeekPosition(summary)}
				businessColor={page.businessColor}
				taskTypeColor={page.taskTypeColor}
				childProgressLabel={text.task.relationships.progressLabel}
			/>
			{/if}
		</Tabs.Content>
		<Tabs.Content value="list" class="min-h-[36rem]">
			{#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p><button class="mt-2 text-sm underline" onclick={ensureFullState}>{text.retryHistory}</button>{/if}
			{#if summary?.completeness === 'full' && hasShownWeek}
			<TaskListView
				emptyLabel={page.tasks().length > 0 ? text.task.empty : text.task.noTasks}
				showEmpty={!isPendingRead && !isLoading && !loadError && !historyError}
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
				canUpdateTask={isLoading ? () => false : page.canUpdateTask}
				{focusedTaskID}
			/>
			{:else if !historyError && !loadError}
				<TaskContentSkeleton label={text.loadingHistory} {text} />
			{/if}
		</Tabs.Content>
		<Tabs.Content value="roadmap" class="min-h-[36rem]">
			{#if taskViewTab === 'roadmap'}
			{#if historyError}<p role="alert" class="text-sm text-destructive">{historyError}</p><button class="mt-2 text-sm underline" onclick={ensureFullState}>{text.retryHistory}</button>{/if}
			{#if summary?.completeness === 'full' || relationshipSnapshotTasks}
			{@const isHistoryFresh = summary?.completeness === 'full'}
			<TaskRoadmap
				tasks={page.filters.tasks(relationshipTasks)}
				allTasks={relationshipTasks}
				serverChildProgress={isHistoryFresh ? summary?.childProgressByParent : undefined}
				centerMonth={currentRoadmapMonth}
				month={roadmapMonth}
				{todayISO}
				text={text.task}
				statusLabel={page.statusLabel}
				businessColor={page.businessColor}
				openTask={page.openTask}
				canUpdateTask={isLoading || !isHistoryFresh ? () => false : page.canUpdateTask}
				updateTaskDates={page.updateTaskDates}
				selectMonth={(month) => { roadmapMonth = month; }}
			/>
			{:else if !historyError && !loadError}
				<TaskRoadmapSkeleton label={text.task.roadmap.loading} />
			{/if}
			{/if}
		</Tabs.Content>
	</Tabs.Root>
</div>

<TaskChildCompletionDialog question={page.board.childCompletionQuestion} text={text.task.childCompletion} />

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
	tasks={relationshipTasks}
	currentMemberID={page.currentMemberID()}
	pendingRelationshipTaskIDs={page.relationships.pendingTaskIDs}
	memberEmail={page.memberEmail}
	bind:taskDraft={page.editor.taskDraft}
	bind:draftChildTaskIDs={page.editor.draftChildTaskIDs}
	isEditingTask={page.editor.isEditingTask}
	isEditingLocked={isLoading}
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
	hasRelationshipData={summary?.completeness === 'full' || relationshipSnapshotTasks !== null}
	relationshipsReady={summary?.completeness === 'full' && !isHistoryLoading && !isLoading}
	relationshipsLoadingLabel={text.loadingHistory}
	relationshipsError={historyError}
	relationshipsRetryLabel={text.retryHistory}
	loadRelationships={ensureFullState}
/>
