<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount, untrack } from 'svelte';
	import TaskTabRow from './task-tab-row.svelte';
	import TasksView from './tasks-view.svelte';
	import TaskContentSkeleton from './task-content-skeleton.svelte';
	import { Spinner } from '$lib/components/ui/spinner';
	import { taskWeeklySummaryOf, mergeTaskSummary, fetchTaskState, forgetTaskStateRead } from './task-api';
	import { taskWeekForCode } from '$lib/task/task-week-code';
	import { taskBoardState } from '$lib/task/task-state';
	import { ToolRefused } from '$lib/public-api-call';
	import { subscribeTaskWrites } from '$lib/task/task-live-refresh';
	import { supersededLoadIsNotAFailure, type TaskLoadOptions } from './task-load-tracker';
	import { createTaskReadSession, sameTaskReadContext, type TaskReadContext } from './task-read-session';
	import { rememberTask, forgetLastSeenTask } from './task-last-seen';
	import { clearStoredTaskSnapshot, taskSnapshotGeneration, subscribeTaskSnapshotInvalidation } from './task-snapshot-storage';
	import type { TaskState, TaskSummary } from './task-types';
	import { taskText } from './text';

	let { data } = $props();
	let summary = $state<TaskSummary | null>(null);
	let taskState = $state<TaskState | null>(null);
	let taskScope = untrack(() => data.taskScope);
	let isDisposed = false;
	let selectedWeek = $state('');
	let lastTaskQuery: string | undefined;
	let activeTab = $state('tasks');
	let TaskDefinitionsEditor = $state<typeof import('./task-definitions-editor.svelte').default | null>(null);
	let TaskMembersView = $state<typeof import('./task-members-view.svelte').default | null>(null);
	let TaskReportView = $state<typeof import('./task-report-view.svelte').default | null>(null);
	let createReportSections = $state<typeof import('./task-report-sections-model').createTaskReportSections | null>(null);
	let paneError = $state('');
	let pendingTaskID = $state('');
	let focusedTaskID = $state('');
	let isLoading = $state(true);
	let errorMessage = $state('');
	let cacheSavedAt = $state(0);
	let isRefreshing = $state(false);
	let isHistoryLoading = $state(false);
	let historyError = $state('');
	type PendingTaskRead = TaskReadContext & { promise: Promise<boolean> };
	let fullRequest: PendingTaskRead | null = null;
	let boardRequest: PendingTaskRead | null = null;
	let historyAfterPeople: PendingTaskRead | null = null;
	let stateReadGeneration = $state(-1);
	let isStateFresh = $derived(stateReadGeneration === taskSnapshotGeneration() && !cacheSavedAt);

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const weeklyTasks = () => summary?.weeklyTasks ?? tasks();
	const definitions = () =>
		summary?.definitions ?? {
			categories: [],
			types: [],
			sizes: []
		};
	const text = createPageText(taskText);
	const reportSections = () => summary && createReportSections ? createReportSections({
		metrics: summary.metrics,
		text,
		summary,
		tasks: weeklyTasks(),
		members: members(),
		definitions: definitions()
	}) : null;
	let sections = $derived(reportSections());
	const readSession = createTaskReadSession();

	$effect(() => {
		const taskRead = data.taskRead;
		const taskBoardRead = data.taskBoardRead;
		const scope = data.taskScope;
		const week = data.taskWeek?.code ?? '';
		const cached = data.lastTask;
		const readGeneration = data.taskReadGeneration;
		let restoreFrame = 0;
		untrack(() => {
			const scopeChanged = taskScope !== scope;
			const requestedWeek = page.url.searchParams.get('week') ?? '';
			const shownWeek = scopeChanged || !selectedWeek ? week : taskWeekForCode(requestedWeek || selectedWeek, new Date()).code;
			if (scopeChanged) {
				taskScope = scope;
				focusedTaskID = '';
				activeTab = 'tasks';
				taskState = null;
				summary = null;
				cacheSavedAt = 0;
				fullRequest = null;
				historyError = '';
				paneError = '';
				pendingTaskID = page.url.searchParams.get('task') ?? '';
			}
			selectedWeek = shownWeek;
			readSession.select(scope, shownWeek);
			fullRequest = null;
			boardRequest = null;
			historyAfterPeople = null;
			isHistoryLoading = false;
			if (taskRead && readGeneration !== undefined) {
				if (readSession.needsFullHistory()) {
					stateReadGeneration = -1;
					void ensureFullState();
				} else if (shownWeek !== week) {
					void loadTask(shownWeek, { reloadState: false });
				} else {
					const promise = applyTaskRead(taskRead, scope, readGeneration, week, false, {}, taskBoardRead);
					const request = { scope, generation: readGeneration, week, promise };
					boardRequest = request;
					void promise.finally(() => { if (boardRequest === request) boardRequest = null; });
				}
			}
			if (!summary && shownWeek === week && cached?.state && cached.summary) restoreFrame = requestAnimationFrame(() => {
				if (isDisposed || summary || taskState || scope !== taskScope || readGeneration !== taskSnapshotGeneration()) return;
				taskState = cached.state;
				summary = cached.summary;
				cacheSavedAt = cached.savedAt ?? 0;
				isLoading = false;
			});
		});
		return () => { cancelAnimationFrame(restoreFrame); readSession.invalidate(); };
	});

	$effect(() => {
		const week = page.url.searchParams.get('week') ?? '';
		const requestedTaskID = page.url.searchParams.get('task') ?? '';
		untrack(() => {
			if (lastTaskQuery !== requestedTaskID) {
				pendingTaskID = requestedTaskID;
				lastTaskQuery = requestedTaskID;
				openPendingTask();
			}
			if (week && taskWeekForCode(week, new Date()).code !== selectedWeek) void loadTask(week, { reloadState: false });
		});
	});

	onMount(() => {
		const stopFollowingInvalidation = subscribeTaskSnapshotInvalidation(() => { stateReadGeneration = -1; });
		const stopFollowingTaskWrites = subscribeTaskWrites(() => { void refreshCurrentWeek(); });
		return () => {
			isDisposed = true;
			readSession.invalidate();
			stopFollowingTaskWrites();
			stopFollowingInvalidation();
		};
	});

	async function loadTask(week: string, options: TaskLoadOptions = {}): Promise<boolean> {
		const shown = taskWeekForCode(week, new Date());
		const weekChanged = selectedWeek !== shown.code;
		selectedWeek = shown.code;
		replaceWeekQuery(shown.code);
		readSession.select(taskScope, selectedWeek);
		if (weekChanged) {
			fullRequest = null;
			boardRequest = null;
			historyAfterPeople = null;
			isHistoryLoading = false;
		}
		if (options.reloadState === false && taskState?.completeness === 'full' && stateReadGeneration === taskSnapshotGeneration() && !cacheSavedAt) {
			applyTaskState(taskState);
			return true;
		}
		if (options.reloadState !== false) {
			forgetTaskStateRead(taskScope);
			clearStoredTaskSnapshot(taskScope);
			fullRequest = null;
			boardRequest = null;
			historyAfterPeople = null;
		}
		if (readSession.needsFullHistory()) return ensureFullState();
		const generation = taskSnapshotGeneration();
		const reading = readTask(taskScope, shown.startISO);
		const preview = data.taskViewer ? taskBoardState(taskScope, data.taskViewer, shown.startISO).catch(() => null) : null;
		const promise = applyTaskRead(reading, taskScope, generation, shown.code, false, options, preview);
		const request = { scope: taskScope, generation, week: shown.code, promise };
		boardRequest = request;
		void promise.finally(() => { if (boardRequest === request) boardRequest = null; });
		return promise;
	}

	function readTask(scope: string, boardWeek?: string) {
		return fetchTaskState(scope, boardWeek).then(
			state => ({ state, error: '', denied: false }),
			(error: unknown) => ({ state: null, error: error instanceof Error ? error.message : String(error), denied: error instanceof ToolRefused && (error.status === 401 || error.status === 403) })
		);
	}

	function ensureFullState(): Promise<boolean> {
		readSession.requireFullHistory();
		if (taskState?.completeness === 'full' && stateReadGeneration === taskSnapshotGeneration() && !cacheSavedAt) return Promise.resolve(true);
		const generation = taskSnapshotGeneration();
		const context = { scope: taskScope, generation, week: selectedWeek };
		if (boardRequest && sameTaskReadContext(boardRequest, context) && !taskState?.peopleReady) {
			if (historyAfterPeople && sameTaskReadContext(historyAfterPeople, context)) return historyAfterPeople.promise;
			const scope = taskScope;
			const week = selectedWeek;
			isHistoryLoading = true;
			const promise = boardRequest.promise.then(ready => {
				if (isDisposed || scope !== taskScope || week !== selectedWeek || generation !== taskSnapshotGeneration()) return false;
				if (!ready) { historyError = errorMessage || text.loadError; return false; }
				return ensureFullState();
			});
			const request = { scope, generation, week, promise };
			historyAfterPeople = request;
			void promise.finally(() => { if (historyAfterPeople === request) historyAfterPeople = null; });
			return promise;
		}
		if (fullRequest && sameTaskReadContext(fullRequest, context)) return fullRequest.promise;
		const promise = applyTaskRead(readTask(taskScope), taskScope, generation, selectedWeek, true, { preserveActiveTabOnError: true });
		const request = { scope: taskScope, generation, week: selectedWeek, promise };
		fullRequest = request;
		void promise.finally(() => { if (fullRequest === request) fullRequest = null; });
		return promise;
	}

	async function applyTaskRead(taskRead: NonNullable<typeof data.taskRead>, scope: string, readGeneration: number, week: string, full: boolean, options: TaskLoadOptions = {}, taskBoardRead: typeof data.taskBoardRead = null): Promise<boolean> {
		const ticket = readSession.start(readGeneration);
		let settled = false;
		const isCurrent = () => !isDisposed && readSession.isCurrent(ticket, taskSnapshotGeneration()) && scope === taskScope && week === selectedWeek;
		isLoading = !full || !taskState;
		isHistoryLoading = full;
		isRefreshing = true;
		errorMessage = '';
		if (full) historyError = '';
		if (taskBoardRead) void taskBoardRead.then(board => {
			if (!board || settled || !isCurrent()) return;
			stateReadGeneration = readGeneration;
			cacheSavedAt = 0;
			applyTaskState(board);
			isLoading = false;
		});
		try {
			const loaded = await taskRead;
			settled = true;
			if (!isCurrent()) return supersededLoadIsNotAFailure;
			if (loaded.denied) {
				forgetLastSeenTask(scope);
				taskState = null;
				summary = null;
				cacheSavedAt = 0;
				stateReadGeneration = -1;
				errorMessage = loaded.error || text.loadError;
				if (full) historyError = errorMessage;
				return false;
			}
			if (!loaded.state && !taskState) summary = null;
			if (!loaded.state) throw new Error(loaded.error || text.loadError);
			stateReadGeneration = readGeneration;
			applyTaskState(loaded.state, true);
			return true;
		} catch (error) {
			if (!isCurrent()) return supersededLoadIsNotAFailure;
			const message = error instanceof Error ? error.message : text.loadError;
			if (full) historyError = message;
			else errorMessage = message;
			if (!taskState) summary = null;
			if (!options.preserveActiveTabOnError) activeTab = 'tasks';
			return false;
		} finally {
			settled = true;
			if (readSession.owns(ticket)) { isLoading = false; isRefreshing = false; isHistoryLoading = false; }
		}
	}

	function applyTaskState(nextState: TaskState, fresh = false): void {
		taskState = nextState;
		summary = mergeTaskSummary(nextState, taskWeeklySummaryOf(nextState, selectedWeek));
		if (fresh) {
			rememberTask(nextState, summary, taskScope, stateReadGeneration);
			cacheSavedAt = 0;
		}
		openPendingTask();
		if (summary.week.code) replaceWeekQuery(summary.week.code);
	}

	function openPendingTask() {
		if (!pendingTaskID || !taskState || !summary || cacheSavedAt || stateReadGeneration !== taskSnapshotGeneration()) return;
		const task = summary.tasks.find((candidate) => candidate.id === pendingTaskID);
		if (!task && taskState.completeness !== 'full') {
			if (!historyError) void ensureFullState();
			return;
		}
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

	function openTaskWhenReady(taskID: string): void {
		pendingTaskID = taskID;
		openPendingTask();
	}

	function selectTab(tab: string) {
		activeTab = tab;
		paneError = '';
		if (tab === 'report' || tab === 'members') void ensureFullState();
		const scope = taskScope;
		void loadPane(tab).catch(error => { if (scope === taskScope && activeTab === tab) paneError = error instanceof Error ? error.message : text.loadError; });
	}

	async function loadPane(tab: string): Promise<void> {
		if (tab === 'report' && !TaskReportView) {
			const [view, model] = await Promise.all([import('./task-report-view.svelte'), import('./task-report-sections-model')]);
			createReportSections = model.createTaskReportSections;
			TaskReportView = view.default;
		}
		if (tab === 'members' && !TaskMembersView) TaskMembersView = (await import('./task-members-view.svelte')).default;
		if (tab === 'definitions' && !TaskDefinitionsEditor) TaskDefinitionsEditor = (await import('./task-definitions-editor.svelte')).default;
	}

	function refreshCurrentWeek() {
		return loadTask(selectedWeek || currentWeek(), { reloadState: true });
	}

	$effect(() => pageActions.setRefresh(async () => { await refreshCurrentWeek(); }));


</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main data-task-ready={!isLoading && !errorMessage && isStateFresh} data-task-progress={isLoading ? summary ? 'tasks' : 'skeleton' : cacheSavedAt ? 'snapshot' : 'ready'} class="min-h-screen min-w-0 flex-1 bg-background text-foreground">
	<div class="flex w-full min-w-0 flex-col gap-6 px-4 py-6 md:px-8">
		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<TaskTabRow activeTab={activeTab} labels={text.tabs} disabled={isLoading || !isStateFresh} onSelectTab={selectTab} />
		{#if summary && !summary.peopleReady && isRefreshing}
			<p role="status" class="flex items-center gap-2 text-sm text-muted-foreground"><Spinner class="size-4" /><span class="sr-only">{text.preparingPeople}</span></p>
		{/if}
		{#if cacheSavedAt && (isRefreshing || errorMessage)}
			<p role="status" data-task-cache-status class="flex items-center gap-2 text-sm text-muted-foreground">{#if isRefreshing}<Spinner class="size-4" />{/if}{errorMessage ? text.cachedUnavailable : text.cachedRefreshing}</p>
		{/if}
		{#if pendingTaskID && historyError}
			<p role="status" class="text-sm text-destructive">{historyError}</p>
			<button class="text-left text-sm underline" onclick={ensureFullState}>{text.retryHistory}</button>
		{/if}

		{#key data.taskScope}
		<div class={activeTab === 'tasks' ? 'flex flex-col gap-6' : 'hidden'}>
			<TasksView
				{summary}
				shownWeek={selectedWeek ? taskWeekForCode(selectedWeek, new Date()) : summary?.week}
				canNavigateWeek={Boolean(summary) && !cacheSavedAt}
				{focusedTaskID}
				{openTaskWhenReady}
				{text}
				isLoading={isLoading || !isStateFresh}
				isPendingRead={isLoading || isRefreshing || isHistoryLoading}
				loadError={errorMessage}
				{ensureFullState}
				{isHistoryLoading}
				{historyError}
				{loadTask}
				{selectWeek}
				setPageErrorMessage={(message) => {
					errorMessage = message;
				}}
			/>
		</div>
		<div class={activeTab === 'report' ? '' : 'hidden'}>
			{#if TaskReportView && sections && summary?.completeness === 'full' && summary.week.code === selectedWeek}
				<div aria-busy={isRefreshing || isHistoryLoading}>
					<TaskReportView {sections} {summary} text={text.report} />
				</div>
			{/if}
			{#if activeTab === 'report'}
				{#if paneError || historyError}<p role="alert" class="text-sm text-destructive">{paneError || historyError}</p><button class="mt-2 text-sm underline" onclick={() => selectTab('report')}>{text.retryHistory}</button>
				{:else if !TaskReportView || summary?.completeness !== 'full' || summary.week.code !== selectedWeek}<TaskContentSkeleton variant="report" label={text.loadingHistory} />{/if}
			{/if}
		</div>
		<div class={activeTab === 'definitions' ? '' : 'hidden'}>
			{#if TaskDefinitionsEditor}
			<TaskDefinitionsEditor
				{summary}
				isFresh={isStateFresh && !isLoading}
				loadError={errorMessage || text.loadError}
				text={text.definitions}
				{loadTask}
			/>
			{:else if activeTab === 'definitions'}
				{#if paneError}<p role="alert" class="text-sm text-destructive">{paneError}</p><button class="mt-2 text-sm underline" onclick={() => selectTab('definitions')}>{text.retryHistory}</button>
				{:else}<TaskContentSkeleton variant="definitions" label={text.loading} />{/if}
			{/if}
		</div>
		<div class={activeTab === 'members' ? '' : 'hidden'}>
			{#if TaskMembersView && summary?.completeness === 'full' && summary.week.code === selectedWeek}
				<div aria-busy={isRefreshing || isHistoryLoading}>
					<TaskMembersView members={members()} text={text.members} />
				</div>
			{/if}
			{#if activeTab === 'members'}
				{#if paneError || historyError}<p role="alert" class="text-sm text-destructive">{paneError || historyError}</p><button class="mt-2 text-sm underline" onclick={() => selectTab('members')}>{text.retryHistory}</button>
				{:else if !TaskMembersView || summary?.completeness !== 'full' || summary.week.code !== selectedWeek}<TaskContentSkeleton variant="members" label={text.loadingHistory} />{/if}
			{/if}
		</div>
		{/key}
	</div>
</main>
