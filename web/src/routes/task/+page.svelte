<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { replaceState } from '$app/navigation';
	import { onMount } from 'svelte';
	import TaskDefinitionsEditor from './task-definitions-editor.svelte';
	import TaskMembersView from './task-members-view.svelte';
	import TaskReportView from './task-report-view.svelte';
	import TaskTabRow from './task-tab-row.svelte';
	import TasksView from './tasks-view.svelte';
	import { fetchTaskState, fetchTaskWeeklySummary, mergeTaskSummary } from './task-api';
	import { subscribeTaskWrites } from '$lib/task/task-live-refresh';
	import { createTaskLoadTracker, type TaskLoadOptions } from './task-load-tracker';
	import { lastSeenTask, rememberTask } from './task-last-seen';
	import { createTaskReportSections, emptyTaskMetrics } from './task-report-sections-model';
	import type { TaskState, TaskSummary, TaskWeeklySummary } from './task-types';
	import { taskText } from './text';

	const lastSeen = lastSeenTask();
	let summary = $state<TaskSummary | null>(lastSeen.summary);
	let taskState = $state<TaskState | null>(lastSeen.state);
	let activeTab = $state('tasks');
	let pendingTaskID = $state('');
	let focusedTaskID = $state('');
	let isLoading = $state(true);
	let errorMessage = $state('');
	const weeklySummaryCache = new Map<string, TaskWeeklySummary>();

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

	onMount(() => {
		const params = new URLSearchParams(location.search);
		const week = params.get('week') ?? '';
		pendingTaskID = params.get('task') ?? '';
		loadTask(week, { reloadState: true });
		return subscribeTaskWrites(() => { void refreshCurrentWeek(); });
	});

	async function loadTask(week: string, options: TaskLoadOptions = {}): Promise<boolean> {
		const loadID = taskLoadTracker.start();
		const reloadState = options.reloadState ?? true;
		isLoading = !summary;
		errorMessage = '';
		try {
			if (reloadState) weeklySummaryCache.clear();
			const [nextState, weeklySummary] = await Promise.all([
				reloadState || !taskState ? fetchTaskState() : Promise.resolve(taskState),
				fetchCachedTaskWeeklySummary(week)
			]);
			if (!taskLoadTracker.isCurrent(loadID)) return false;
			taskState = nextState;
			summary = mergeTaskSummary(nextState, weeklySummary);
			rememberTask(nextState, summary);
			openPendingTask();
			if (summary.week.code) replaceWeekQuery(summary.week.code);
			return true;
		} catch (error) {
			if (!taskLoadTracker.isCurrent(loadID)) return false;
			errorMessage = error instanceof Error ? error.message : text.loadError;
			if (!taskState) summary = null;
			if (!options.preserveActiveTabOnError) activeTab = 'tasks';
			return false;
		} finally {
			if (taskLoadTracker.isCurrent(loadID)) isLoading = false;
		}
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

	function selectCurrentWeek() {
		loadTask(summary?.currentWeek?.code ?? '', { reloadState: false });
	}

	function selectWeek(week: string) {
		loadTask(week, { reloadState: false });
	}

	function refreshCurrentWeek() {
		return loadTask(currentWeek(), { reloadState: true });
	}

	$effect(() => pageActions.setRefresh(async () => { await refreshCurrentWeek(); }));

	async function fetchCachedTaskWeeklySummary(week: string): Promise<TaskWeeklySummary> {
		const cachedSummary = week ? weeklySummaryCache.get(week) : undefined;
		if (cachedSummary) return cachedSummary;
		const weeklySummary = await fetchTaskWeeklySummary(week);
		if (weeklySummary.week.code) weeklySummaryCache.set(weeklySummary.week.code, weeklySummary);
		return weeklySummary;
	}

</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main data-task-ready={!isLoading && !errorMessage} class="min-h-screen min-w-0 flex-1 bg-background text-foreground">
	<div class="flex w-full min-w-0 flex-col gap-6 px-4 py-6 md:px-8">
		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<TaskTabRow activeTab={activeTab} labels={text.tabs} onSelectTab={(value) => (activeTab = value)} />

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
			<TaskReportView sections={reportSections()} {summary} text={text.report} />
		</div>
		<div class={activeTab === 'definitions' ? '' : 'hidden'}>
			<TaskDefinitionsEditor
				{summary}
				loadError={errorMessage || text.loadError}
				text={text.definitions}
				{loadTask}
			/>
		</div>
		<div class={activeTab === 'members' ? '' : 'hidden'}>
			<TaskMembersView members={members()} text={text.members} />
		</div>
	</div>
</main>
