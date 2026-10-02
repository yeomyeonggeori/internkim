<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { invalidate, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount, untrack } from 'svelte';
	import TaskDefinitionsEditor from './task-definitions-editor.svelte';
	import TaskMembersView from './task-members-view.svelte';
	import TaskReportView from './task-report-view.svelte';
	import TaskTabRow from './task-tab-row.svelte';
	import TasksView from './tasks-view.svelte';
	import { taskWeeklySummaryOf, mergeTaskSummary, taskStateDependency, forgetTaskStateRead } from './task-api';
	import { subscribeTaskWrites } from '$lib/task/task-live-refresh';
	import { createTaskLoadTracker, type TaskLoadOptions } from './task-load-tracker';
	import { rememberTask, forgetLastSeenTask } from './task-last-seen';
	import { clearStoredTaskSnapshot, taskSnapshotGeneration } from './task-snapshot-storage';
	import { createTaskReportSections, emptyTaskMetrics } from './task-report-sections-model';
	import type { TaskState, TaskSummary } from './task-types';
	import { taskText } from './text';

	let { data } = $props();
	let summary = $state<TaskSummary | null>(null);
	let taskState = $state<TaskState | null>(null);
	let taskScope = untrack(() => data.taskScope);
	let isDisposed = false;
	let selectedWeek = '';
	let lastWeekQuery: string | undefined;
	let lastTaskQuery: string | undefined;
	let activeTab = $state('tasks');
	let loadedTabs = $state(['tasks']);
	let pendingTaskID = $state('');
	let focusedTaskID = $state('');
	let isLoading = $state(true);
	let errorMessage = $state('');
	let cacheSavedAt = $state(0);
	let isRefreshing = $state(false);

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const weeklyTasks = () => summary?.weeklyTasks ?? tasks();
	const metrics = () => summary?.metrics ?? emptyTaskMetrics;
	const definitions = () =>
		summary?.definitions ?? {
			categories: [],
			types: [],
			sizes: []
		};
	const text = createPageText(taskText);
	const reportSections = () => createTaskReportSections({
		metrics: metrics(),
		text,
		summary,
		tasks: weeklyTasks(),
		members: members(),
		definitions: definitions()
	});
	const taskLoadTracker = createTaskLoadTracker();

	$effect(() => {
		const taskRead = data.taskRead;
		const taskBoardRead = data.taskBoardRead;
		const scope = data.taskScope;
		const week = page.url.searchParams.get('week') ?? '';
		const requestedTaskID = page.url.searchParams.get('task') ?? '';
		let restoreFrame = 0;
		untrack(() => {
			const scopeChanged = taskScope !== scope;
			if (scopeChanged) {
				taskScope = scope;
				focusedTaskID = '';
				activeTab = 'tasks';
				loadedTabs = ['tasks'];
				taskState = null;
				summary = null;
				cacheSavedAt = 0;
			}
			const weekChanged = scopeChanged || lastWeekQuery !== week;
			if (scopeChanged || lastTaskQuery !== requestedTaskID) pendingTaskID = requestedTaskID;
			lastWeekQuery = week;
			lastTaskQuery = requestedTaskID;
			if (weekChanged) selectedWeek = week;
			if (taskRead) void applyTaskRead(taskRead, {}, taskBoardRead);
			const cached = data.lastTask;
			const readGeneration = data.taskReadGeneration;
			if (!summary && cached?.state && cached.summary) restoreFrame = requestAnimationFrame(() => {
				if (isDisposed || summary || taskState || scope !== taskScope || readGeneration !== taskSnapshotGeneration()) return;
				taskState = cached.state;
				summary = cached.summary;
				cacheSavedAt = cached.savedAt ?? 0;
				isLoading = false;
			});
		});
		return () => { cancelAnimationFrame(restoreFrame); taskLoadTracker.start(); };
	});

	onMount(() => {
		const stopFollowingTaskWrites = subscribeTaskWrites(() => { void refreshCurrentWeek(); });
		return () => {
			isDisposed = true;
			taskLoadTracker.start();
			stopFollowingTaskWrites();
		};
	});

	async function loadTask(week: string, options: TaskLoadOptions = {}): Promise<boolean> {
		selectedWeek = week;
		if (options.reloadState === false && taskState) {
			applyTaskState(taskState);
			return true;
		}
		forgetTaskStateRead(taskScope);
		clearStoredTaskSnapshot(taskScope);
		await invalidate(taskStateDependency);
		if (isDisposed) return false;
		return data.taskRead ? applyTaskRead(data.taskRead, options) : false;
	}

	async function applyTaskRead(taskRead: NonNullable<typeof data.taskRead>, options: TaskLoadOptions = {}, taskBoardRead = data.taskBoardRead): Promise<boolean> {
		const loadID = taskLoadTracker.start();
		const scope = taskScope;
		const readGeneration = data.taskReadGeneration;
		let settled = false;
		isLoading = !summary;
		isRefreshing = true;
		errorMessage = '';
		if (!summary && taskBoardRead) void taskBoardRead.then(board => {
			if (!board || settled || summary || isDisposed || !taskLoadTracker.isCurrent(loadID) || scope !== taskScope || readGeneration !== taskSnapshotGeneration()) return;
			summary = mergeTaskSummary(board, taskWeeklySummaryOf(board, selectedWeek));
		});
		try {
			const loaded = await taskRead;
			settled = true;
			if (isDisposed || !taskLoadTracker.isCurrent(loadID) || scope !== taskScope) return false;
			if (loaded.denied) {
				forgetLastSeenTask(scope);
				taskState = null;
				summary = null;
				cacheSavedAt = 0;
			}
			if (!loaded.state && !taskState) summary = null;
			if (!loaded.state) throw new Error(loaded.error || text.loadError);
			if (readGeneration !== taskSnapshotGeneration()) {
				if (!taskState) summary = null;
				throw new Error(text.loadError);
			}
			applyTaskState(loaded.state, true);
			return true;
		} catch (error) {
			if (!taskLoadTracker.isCurrent(loadID)) return false;
			errorMessage = error instanceof Error ? error.message : text.loadError;
			if (!taskState) summary = null;
			if (!options.preserveActiveTabOnError) activeTab = 'tasks';
			return false;
		} finally {
			settled = true;
			if (taskLoadTracker.isCurrent(loadID)) { isLoading = false; isRefreshing = false; }
		}
	}

	function applyTaskState(nextState: TaskState, fresh = false): void {
		taskState = nextState;
		summary = mergeTaskSummary(nextState, taskWeeklySummaryOf(nextState, selectedWeek));
		if (fresh) {
			rememberTask(nextState, summary, taskScope);
			cacheSavedAt = 0;
		}
		openPendingTask();
		if (summary.week.code) replaceWeekQuery(summary.week.code);
	}

	function openPendingTask() {
		if (!pendingTaskID || !summary) return;
		const task = summary.tasks.find((candidate) => candidate.id === pendingTaskID);
		pendingTaskID = '';
		if (!task) return;
		focusedTaskID = task.id;
		activeTab = 'tasks';
	}

	function replaceWeekQuery(week: string) {
		const url = new URL(location.href);
		url.searchParams.set('week', week);
		replaceState(url, {});
	}

	function selectWeek(week: string) {
		loadTask(week, { reloadState: false });
	}

	function selectTab(tab: string) {
		activeTab = tab;
		if (!loadedTabs.includes(tab)) loadedTabs.push(tab);
	}

	function refreshCurrentWeek() {
		return loadTask(currentWeek(), { reloadState: true });
	}

	$effect(() => pageActions.setRefresh(async () => { await refreshCurrentWeek(); }));


</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main data-task-ready={!isLoading && !errorMessage} data-task-progress={isLoading ? summary ? 'tasks' : 'skeleton' : cacheSavedAt ? 'snapshot' : 'ready'} class="min-h-screen min-w-0 flex-1 bg-background text-foreground">
	<div class="flex w-full min-w-0 flex-col gap-6 px-4 py-6 md:px-8">
		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<TaskTabRow activeTab={activeTab} labels={text.tabs} disabled={isLoading} onSelectTab={selectTab} />
		{#if isLoading && summary}
			<p role="status" class="text-sm text-muted-foreground">{text.preparingPeople}</p>
		{/if}
		{#if cacheSavedAt && (isRefreshing || errorMessage)}
			<p role="status" data-task-cache-status class="text-sm text-muted-foreground">{errorMessage ? text.cachedUnavailable : text.cachedRefreshing}</p>
		{/if}

		{#key data.taskScope}
		<div class={activeTab === 'tasks' ? 'flex flex-col gap-6' : 'hidden'}>
			<TasksView
				{summary}
				{focusedTaskID}
				{text}
				{isLoading}
				{loadTask}
				{selectWeek}
				setPageErrorMessage={(message) => {
					errorMessage = message;
				}}
			/>
		</div>
		<div class={activeTab === 'report' ? '' : 'hidden'}>
			{#if loadedTabs.includes('report')}
			<TaskReportView sections={reportSections()} {summary} text={text.report} />
			{/if}
		</div>
		<div class={activeTab === 'definitions' ? '' : 'hidden'}>
			{#if loadedTabs.includes('definitions')}
			<TaskDefinitionsEditor
				{summary}
				loadError={errorMessage || text.loadError}
				text={text.definitions}
				{loadTask}
			/>
			{/if}
		</div>
		<div class={activeTab === 'members' ? '' : 'hidden'}>
			{#if loadedTabs.includes('members')}
			<TaskMembersView members={members()} text={text.members} />
			{/if}
		</div>
		{/key}
	</div>
</main>
