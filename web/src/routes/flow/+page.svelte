<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { ConfirmDeleteDialog } from '$lib/components/ui/confirm-delete-dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { replaceState } from '$app/navigation';
	import { onMount } from 'svelte';
	import FlowDefinitionsEditor from './flow-definitions-editor.svelte';
	import FlowMembersView from './flow-members-view.svelte';
	import FlowReportView from './flow-report-view.svelte';
	import FlowTabRow from './flow-tab-row.svelte';
	import FlowTasksView from './flow-tasks-view.svelte';
	import { fetchFlowState, fetchFlowWeeklySummary, mergeFlowSummary } from './flow-api';
	import { createFlowLoadTracker, type FlowLoadOptions } from './flow-load-tracker';
	import { createFlowReportSections, emptyFlowMetrics } from './flow-report-sections-model';
	import type { FlowState, FlowSummary, FlowWeeklySummary } from './flow-types';
	import { flowText } from './text';

	let summary = $state<FlowSummary | null>(null);
	let flowState = $state<FlowState | null>(null);
	let activeTab = $state('tasks');
	let pendingTaskID = $state('');
	let focusedTaskID = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');
	const weeklySummaryCache = new Map<string, FlowWeeklySummary>();

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const weeklyTasks = () => summary?.weeklyTasks ?? tasks();
	const metrics = () => summary?.metrics ?? emptyFlowMetrics;
	const definitions = () =>
		summary?.definitions ?? {
			categories: [],
			types: [],
			sizes: []
		};
	const text = createPageText(flowText);
	const reportSections = () => createFlowReportSections({
		metrics: metrics(),
		text,
		summary,
		tasks: weeklyTasks(),
		members: members(),
		definitions: definitions()
	});
	const flowLoadTracker = createFlowLoadTracker();

	onMount(() => {
		const params = new URLSearchParams(location.search);
		const week = params.get('week') ?? '';
		pendingTaskID = params.get('task') ?? '';
		loadFlow(week, { reloadState: true });
	});

	async function loadFlow(week: string, options: FlowLoadOptions = {}): Promise<boolean> {
		const loadID = flowLoadTracker.start();
		const reloadState = options.reloadState ?? true;
		isLoading = true;
		errorMessage = '';
		try {
			if (reloadState) weeklySummaryCache.clear();
			const [nextState, weeklySummary] = await Promise.all([
				reloadState || !flowState ? fetchFlowState(text.loadError) : Promise.resolve(flowState),
				fetchCachedFlowWeeklySummary(week)
			]);
			if (!flowLoadTracker.isCurrent(loadID)) return false;
			flowState = nextState;
			summary = mergeFlowSummary(nextState, weeklySummary);
			openPendingTask();
			if (summary.week.code) replaceWeekQuery(summary.week.code);
			return true;
		} catch (error) {
			if (!flowLoadTracker.isCurrent(loadID)) return false;
			errorMessage = error instanceof Error ? error.message : text.loadError;
			if (!flowState) summary = null;
			if (!options.preserveActiveTabOnError) activeTab = 'tasks';
			return false;
		} finally {
			if (flowLoadTracker.isCurrent(loadID)) isLoading = false;
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
		loadFlow(summary?.currentWeek?.code ?? '', { reloadState: false });
	}

	function selectWeek(week: string) {
		loadFlow(week, { reloadState: false });
	}

	function refreshCurrentWeek() {
		return loadFlow(currentWeek(), { reloadState: true });
	}

	$effect(() => pageActions.setRefresh(async () => { await refreshCurrentWeek(); }));

	async function fetchCachedFlowWeeklySummary(week: string): Promise<FlowWeeklySummary> {
		const cachedSummary = week ? weeklySummaryCache.get(week) : undefined;
		if (cachedSummary) return cachedSummary;
		const weeklySummary = await fetchFlowWeeklySummary(week, text.loadError);
		if (weeklySummary.week.code) weeklySummaryCache.set(weeklySummary.week.code, weeklySummary);
		return weeklySummary;
	}

</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="min-h-screen min-w-0 flex-1 bg-background text-foreground">
	<div class="flex w-full min-w-0 flex-col gap-6 px-4 py-6 md:px-8">
		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<FlowTabRow activeTab={activeTab} labels={text.tabs} onSelectTab={(value) => (activeTab = value)} />

		<div class={activeTab === 'tasks' ? 'flex flex-col gap-6' : 'hidden'}>
			<FlowTasksView
				{summary}
				{focusedTaskID}
				{text}
				{isLoading}
				{loadFlow}
				{selectWeek}
				setPageErrorMessage={(message) => {
					errorMessage = message;
				}}
			/>
		</div>
		<div class={activeTab === 'report' ? '' : 'hidden'}>
			<FlowReportView sections={reportSections()} {summary} text={text.report} />
		</div>
		<div class={activeTab === 'definitions' ? '' : 'hidden'}>
			<FlowDefinitionsEditor
				{summary}
				loadError={errorMessage || text.loadError}
				text={text.definitions}
				{loadFlow}
			/>
		</div>
		<div class={activeTab === 'members' ? '' : 'hidden'}>
			<FlowMembersView members={members()} text={text.members} />
		</div>
	</div>
</main>

<ConfirmDeleteDialog />
