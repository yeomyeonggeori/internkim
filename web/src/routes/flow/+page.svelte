<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { ConfirmDeleteDialog } from '$lib/components/ui/confirm-delete-dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { replaceState } from '$app/navigation';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { onMount } from 'svelte';
	import FlowDefinitionsEditor from './flow-definitions-editor.svelte';
	import FlowMetricCard from './flow-metric-card.svelte';
	import FlowMembersView from './flow-members-view.svelte';
	import FlowReportView from './flow-report-view.svelte';
	import FlowTabRow from './flow-tab-row.svelte';
	import FlowTasksView from './flow-tasks-view.svelte';
	import FlowWeekSelector from './flow-week-selector.svelte';
	import { fetchFlowState, fetchFlowWeeklySummary, mergeFlowSummary } from './flow-api';
	import { createFlowLoadTracker, type FlowLoadOptions } from './flow-load-tracker';
	import type { FlowMetrics, FlowState, FlowSummary } from './flow-types';
	import { buildFlowReportSections } from './report/flow-report-data';
	import { flowText } from './text';

	const emptyMetrics: FlowMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalDistance: 0,
		statusCounts: {},
		businessCounts: {},
		typeCounts: {},
		memberDistances: {}
	};

	let summary = $state<FlowSummary | null>(null);
	let flowState = $state<FlowState | null>(null);
	let activeTab = $state('report');
	let pendingTaskID = $state('');
	let focusedTaskID = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const weeklyTasks = () => summary?.weeklyTasks ?? tasks();
	const metrics = () => summary?.metrics ?? emptyMetrics;
	const metricMemberDistances = () => {
		const metricValues = metrics();
		return metricValues.memberDistances ?? metricValues.memberScores ?? {};
	};
	const memberDistanceTotal = () => Object.values(metricMemberDistances()).reduce((total, distance) => total + distance, 0);
	const blockedTaskTotal = () => {
		const metricValues = metrics();
		return metricValues.pausedTasks + metricValues.stoppedTasks;
	};
	const definitions = () =>
		summary?.definitions ?? {
			categories: [],
			types: [],
			sizes: []
		};
	const isMemberTab = () => activeTab.startsWith('member:');
	const activeMemberID = () => (isMemberTab() ? activeTab.replace('member:', '') : '');
	const text = createPageText(flowText);
	const reportSections = () =>
		buildFlowReportSections(metrics(), {
			emptyLabel: text.report.empty,
			sectionLabels: {
				weeklyStatus: {
					title: text.report.weeklyStatus,
					description: text.report.weeklyStatusDescription
				},
				memberDistance: {
					title: text.report.memberDistance,
					description: text.report.memberDistanceDescription
				},
				weeklyDistanceTrend: {
					title: text.report.weeklyDistanceTrend,
					description: text.report.weeklyDistanceTrendDescription
				},
				monthlyDistanceTrend: {
					title: text.report.monthlyDistanceTrend,
					description: text.report.monthlyDistanceTrendDescription
				},
				businessDistance: {
					title: text.report.businessDistance,
					description: text.report.businessDistanceDescription
				}
			},
			copy: {
				weekdays: [...text.report.weekdays],
				fallbackType: text.report.fallbackType,
				fallbackBusiness: text.report.fallbackBusiness,
				memberScoreLabel: text.report.memberScoreLabel,
				weeklyScoreLabel: text.report.weeklyScoreLabel,
				monthlyScoreLabel: text.report.monthlyScoreLabel,
				scoreUnit: text.report.scoreUnit,
				teamAverageLabel: text.report.teamAverageLabel,
				memberScrollHint: text.report.memberScrollHint,
				currentWeekTrend: text.report.currentWeekTrend,
				previousWeekTrend: text.report.previousWeekTrend,
				currentMonthTrend: text.report.currentMonthTrend,
				previousMonthTrend: text.report.previousMonthTrend,
				monthlyDayLabelTemplate: text.report.monthlyDayLabelTemplate
			},
			report: summary?.report,
			tasks: weeklyTasks(),
			members: members(),
			definitions: definitions(),
			weekStartISO: summary?.week.startISO
		});
	const hasFlowData = () => summary !== null;
	const canEditDefinitions = () => hasFlowData() && definitions().sizes.length > 0;
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
			const nextState = reloadState || !flowState ? await fetchFlowState(text.loadError) : flowState;
			const weeklySummary = await fetchFlowWeeklySummary(week, text.loadError);
			if (!flowLoadTracker.isCurrent(loadID)) return false;
			flowState = nextState;
			summary = mergeFlowSummary(nextState, weeklySummary);
			openPendingTask();
			if (isMemberTab()) {
				const memberID = activeTab.replace('member:', '');
				if (!members().some((member) => member.id === memberID)) activeTab = 'tasks';
			}
			if (summary.week.code) replaceWeekQuery(summary.week.code);
			return true;
		} catch (error) {
			if (!flowLoadTracker.isCurrent(loadID)) return false;
			errorMessage = error instanceof Error ? error.message : text.loadError;
			if (!flowState) summary = null;
			if (!options.preserveActiveTabOnError) activeTab = 'report';
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
		activeTab = task.participantIDs.includes(task.ownerID) ? `member:${task.ownerID}` : 'tasks';
	}

	function replaceWeekQuery(week: string) {
		const url = new URL(location.href);
		url.searchParams.set('week', week);
		replaceState(url, {});
	}

	function selectCurrentWeek() {
		loadFlow('', { reloadState: false });
	}

	function selectWeek(week: string) {
		loadFlow(week, { reloadState: false });
	}

</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="min-h-screen min-w-0 flex-1 bg-background text-foreground">
	<div class="flex w-full min-w-0 flex-col gap-6 px-4 py-6 md:px-8">
		<header class="flex min-w-0 flex-col gap-4 border-b pb-5 md:flex-row md:items-end md:justify-between">
			<div class="min-w-0 space-y-1">
				<p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.product}</p>
				<h1 class="text-2xl font-semibold">{text.title}</h1>
				<p class="text-sm text-muted-foreground">{text.description}</p>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.previous ?? '')} disabled={!summary || isLoading}>
					<ChevronLeftIcon />
					{text.previousWeek}
				</Button>
				<FlowWeekSelector
					week={summary?.week}
					disabled={!summary || isLoading}
					selectDateLabel={text.selectWeekDate}
					currentWeekLabel={text.currentWeekAction}
					onSelectWeek={selectWeek}
					onSelectCurrentWeek={selectCurrentWeek}
				/>
				<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.next ?? '')} disabled={!summary || isLoading}>
					{text.nextWeek}
					<ChevronRightIcon />
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => loadFlow(currentWeek(), { reloadState: true })} disabled={isLoading}>
					<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
				</Button>
			</div>
		</header>

		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<section class="grid min-w-0 gap-3 md:grid-cols-4">
			<FlowMetricCard label={text.metrics.total} value={metrics().totalTasks} subvalue={`${summary?.week.startISO ?? ''} – ${summary?.week.endISO ?? ''}`} />
			<FlowMetricCard label={text.metrics.completed} value={metrics().completedTasks} subvalue={`${text.metrics.completedDistance} ${memberDistanceTotal()}${text.metrics.distanceUnit}`} />
			<FlowMetricCard label={text.metrics.requested} value={metrics().requestedTasks} subvalue={text.metrics.requestedDescription} />
			<FlowMetricCard label={text.metrics.blocked} value={blockedTaskTotal()} subvalue={text.metrics.blockedDescription} />
		</section>

		<FlowTabRow activeTab={activeTab} labels={text.tabs} members={members()} canEditDefinitions={canEditDefinitions()} onSelectTab={(value) => (activeTab = value)} />

		{#if activeTab === 'report'}
			<FlowReportView sections={reportSections()} />
		{:else if activeTab === 'definitions'}
			<FlowDefinitionsEditor
				{summary}
				loadError={errorMessage || text.loadError}
				text={text.definitions}
				{loadFlow}
			/>
		{:else if activeTab === 'members'}
			<FlowMembersView members={members()} text={text.members} />
		{:else}
			<FlowTasksView
				{summary}
				activeMemberID={activeMemberID()}
				{focusedTaskID}
				{text}
				{loadFlow}
				setPageErrorMessage={(message) => {
					errorMessage = message;
				}}
			/>
		{/if}
	</div>
</main>

<ConfirmDeleteDialog />
