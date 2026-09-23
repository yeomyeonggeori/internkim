<script lang="ts">
	import { page } from '$app/state';
	import { taskListPathOf } from '$lib/app-shell';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Accordion from '$lib/components/ui/accordion';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onDestroy, untrack } from 'svelte';
	import ApprovalDecision from '../approval-decision.svelte';
	import RetryTaskButton from '../retry-task-button.svelte';
	import RawDocument from '../raw-document.svelte';
	import RawLedgerView from '../raw-ledger-view.svelte';
	import { filterLedger, groupLedger } from '../raw-ledger';
	import IntakeDecisionStep from '../intake-decision-step.svelte';
	import TaskStep from '../task-step.svelte';
	import { buildTaskStory } from '../task-story';
	import {
		eventLane,
		fetchServiceLogs,
		fetchTaskDetail,
		formatCostUSD,
		formatEventBody,
		pendingApprovalOf,
		summarizeTimeline,
		taskDetailShareText,
		type TaskDetail,
		type TaskEvent,
		type TimelineSummary
	} from '../runs-api';
	import {
		formatDuration,
		formatLatency,
		formatTaskTimestamp,
		taskStatusBadgeVariant,
		taskStatusIcon,
		taskStatusLabel
	} from '../runs-view';
	import { tasksText } from '../text';

	const text = createPageText(tasksText);
	let detail = $state<TaskDetail | undefined>(undefined);
	let loadError = $state('');
	let selectedTab = $state('story');
	let selectedEventLane = $state('all');
	let eventSearchQuery = $state('');
	let serviceLogLines = $state<string[] | undefined>(undefined);
	let serviceLogsLoading = $state(false);
	let serviceLogsError = $state('');
	let pollTimer: ReturnType<typeof setTimeout> | undefined;
	let loadGeneration = 0;

	const summary = $derived(detail ? summarizeTimeline(detail.taskEvents) : undefined);
	const story = $derived(buildTaskStory(detail?.taskEvents ?? []));
	const runDuration = $derived(detail ? formatDuration(Date.parse(detail.taskRun.updatedAt ?? '') - Date.parse(detail.taskRun.createdAt ?? '')) : '');
	const pendingApproval = $derived(detail ? pendingApprovalOf(detail) : undefined);
	const ledgerSections = $derived(detail ? filterLedger(groupLedger(detail.taskEvents), isEventShown) : []);
	const taskShareText = $derived(detail ? taskDetailShareText(detail) : '');
	const eventLaneFilters = $derived(detail ? buildEventLaneFilters(detail.taskEvents) : []);
	const taskListPath = $derived(taskListPathOf(page.url.pathname));

	async function load(taskRunID: string, generation: number) {
		loadError = '';
		try {
			const nextDetail = await fetchTaskDetail(taskRunID);
			if (generation !== loadGeneration || page.params.id !== taskRunID) return;
			detail = nextDetail;
			if (isActiveTaskRunStatus(nextDetail.taskRun.status)) schedulePoll(taskRunID, generation);
		} catch {
			if (generation !== loadGeneration || page.params.id !== taskRunID) return;
			loadError = text.detailLoadError;
		}
	}

	function schedulePoll(taskRunID: string, generation: number) {
		if (pollTimer) clearTimeout(pollTimer);
		pollTimer = setTimeout(() => void load(taskRunID, generation), 2000);
	}

	function isActiveTaskRunStatus(status: string): boolean {
		return ['planned', 'running', 'waiting_approval', 'waiting_user_input'].includes(status);
	}

	async function loadServiceLogs() {
		serviceLogsLoading = true;
		serviceLogsError = '';
		try {
			const logsResponse = await fetchServiceLogs('blueclaw', page.params.id ?? '');
			serviceLogLines = logsResponse.lines;
		} catch {
			serviceLogsError = text.serviceLogsError;
		} finally {
			serviceLogsLoading = false;
		}
	}

	function isEventShown(taskEvent: TaskEvent): boolean {
		const query = eventSearchQuery.trim().toLowerCase();
		if (selectedEventLane !== 'all' && selectedEventLane !== eventLane(taskEvent.name)) return false;
		if (!query) return true;
		return `${taskEvent.name}\n${formatEventBody(taskEvent.body)}`.toLowerCase().includes(query);
	}

	function buildEventLaneFilters(taskEvents: TaskEvent[]) {
		const counts = taskEvents.reduce(
			(result, taskEvent) => {
				const lane = eventLane(taskEvent.name);
				result[lane] += 1;
				result.all += 1;
				return result;
			},
			{ all: 0, llm: 0, tool: 0, failure: 0, other: 0 }
		);
		return [
			{ value: 'all', label: text.allEvents, count: counts.all },
			{ value: 'llm', label: text.laneLLM, count: counts.llm },
			{ value: 'tool', label: text.laneTool, count: counts.tool },
			{ value: 'failure', label: text.laneFailure, count: counts.failure },
			{ value: 'other', label: text.laneOther, count: counts.other }
		].filter((filter) => filter.value === 'all' || filter.count > 0);
	}

	let openStepValues = $state<string[]>([]);

	const openedFailedStepKeys = new Set<string>();

	$effect(() => {
		const newlyFailedKeys = story.steps.filter((step) => step.isFailed && !openedFailedStepKeys.has(step.key)).map((step) => step.key);
		if (newlyFailedKeys.length === 0) return;
		newlyFailedKeys.forEach((key) => openedFailedStepKeys.add(key));
		openStepValues = [...untrack(() => openStepValues), ...newlyFailedKeys];
	});

	function summaryLine(timelineSummary: TimelineSummary): string {
		return text.summaryLine
			.replace('{calls}', timelineSummary.llmCallCount.toLocaleString())
			.replace('{latency}', formatLatency(timelineSummary.llmLatencyMS))
			.replace('{tokens}', timelineSummary.llmTotalTokens.toLocaleString())
			.replace('{cost}', formatCostUSD(timelineSummary.llmCostUSD))
			.replace('{tools}', timelineSummary.toolCallCount.toLocaleString());
	}

	$effect(() => {
		if (selectedTab !== 'logs') return;
		if (serviceLogLines !== undefined || serviceLogsLoading || serviceLogsError !== '') return;
		void loadServiceLogs();
	});

	$effect(() => {
		const taskRunID = page.params.id ?? '';
		loadGeneration += 1;
		const generation = loadGeneration;
		detail = undefined;
		serviceLogLines = undefined;
		serviceLogsError = '';
		if (pollTimer) clearTimeout(pollTimer);
		void load(taskRunID, generation);
	});

	onDestroy(() => {
		loadGeneration += 1;
		if (pollTimer) clearTimeout(pollTimer);
	});
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="flex min-h-full w-full self-start flex-col gap-5 px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<Button href={taskListPath} variant="ghost" size="sm">
			<ArrowLeftIcon data-icon="inline-start" />
			{text.backToList}
		</Button>
		{#if detail}
			<div class="flex flex-wrap items-center gap-2">
				{#if detail.taskRun.status === 'failed'}
					<RetryTaskButton taskRunID={detail.taskRun.taskRunID} label={text.retryTask} pendingLabel={text.retryingTask} successMessage={text.retrySuccess} errorMessage={text.retryError} />
				{/if}
				<CopyButton text={taskShareText} />
			</div>
		{/if}
	</div>

	{#if loadError}
		<Card.Root size="sm" class="border-destructive/30">
			<Card.Content class="text-sm text-destructive">{loadError}</Card.Content>
		</Card.Root>
	{:else if !detail}
		<section class="flex flex-col gap-3">
			<Skeleton class="h-28 w-full" />
			<Skeleton class="h-64 w-full" />
		</section>
	{:else}
		{@const StatusIcon = taskStatusIcon(detail.taskRun.status)}
		<section class="flex flex-col gap-2">
			<div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
				<Badge variant={taskStatusBadgeVariant(detail.taskRun.status)}>
					<StatusIcon />
					{taskStatusLabel(detail.taskRun.status, text)}
				</Badge>
				{#if detail.taskRun.requesterDisplayName || detail.taskRun.requesterPersonID}
					<span>{displayPersonName(detail.taskRun.requesterDisplayName || detail.taskRun.requesterPersonID)}</span>
					<span aria-hidden="true">·</span>
				{/if}
				<span>{formatTaskTimestamp(detail.taskRun.createdAt)}</span>
				{#if runDuration}
					<span aria-hidden="true">·</span>
					<span>{runDuration}</span>
				{/if}
			</div>
			<h1 class="line-clamp-3 text-lg leading-snug font-semibold">{detail.taskRun.prompt || detail.taskRun.taskRunID}</h1>
			{#if summary}
				<p class="text-xs text-muted-foreground tabular-nums">{summaryLine(summary)}</p>
			{/if}
			{#if detail.taskRun.failureReason}
				<p class="text-sm text-destructive">{detail.taskRun.failureReason}</p>
			{/if}
			{#if detail.taskRun.result}
				<p class="mt-2 rounded-lg bg-muted/50 px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap">{detail.taskRun.result}</p>
			{/if}
			{#if pendingApproval}
				<ApprovalDecision approval={pendingApproval} {text} onDecided={() => void load(page.params.id ?? '', loadGeneration)} />
			{/if}
		</section>

		<UnderlineTabs.Root bind:value={selectedTab} class="min-w-0">
			<UnderlineTabs.List>
				<UnderlineTabs.Trigger value="story">{text.storyTab}</UnderlineTabs.Trigger>
				<UnderlineTabs.Trigger value="timeline">{text.rawTab}</UnderlineTabs.Trigger>
				<UnderlineTabs.Trigger value="logs">{text.logsTab}</UnderlineTabs.Trigger>
			</UnderlineTabs.List>

			<UnderlineTabs.Content value="story" class="min-w-0">
				{#if story.steps.length === 0 && !story.intakeDecision}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noSteps}</p>
				{:else}
					<Accordion.Root type="multiple" bind:value={openStepValues}>
						{#if story.intakeDecision}
							<IntakeDecisionStep decision={story.intakeDecision} isOpen={openStepValues.includes('intake-decision')} {text} />
						{/if}
						{#each story.steps as step (step.key)}
							<TaskStep {step} isOpen={openStepValues.includes(step.key)} {text} />
						{/each}
					</Accordion.Root>
				{/if}
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="timeline" class="flex min-w-0 flex-col gap-3">
				<div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
					<Tabs.Root bind:value={selectedEventLane}>
						<Tabs.List>
							{#each eventLaneFilters as filter (filter.value)}
								<Tabs.Trigger value={filter.value}>{filter.label}</Tabs.Trigger>
							{/each}
						</Tabs.List>
					</Tabs.Root>
					<label class="relative min-w-0 sm:w-64">
						<SearchIcon class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
						<Input bind:value={eventSearchQuery} placeholder={text.searchEvents} class="pl-8" />
					</label>
				</div>
				{#if ledgerSections.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noMatchingEvents}</p>
				{:else}
					<RawLedgerView sections={ledgerSections} {text} />
				{/if}
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="logs" class="flex min-w-0 flex-col gap-3">
				<div class="flex justify-end">
					<Button onclick={loadServiceLogs} disabled={serviceLogsLoading} variant="outline" size="sm">
						<RefreshCwIcon data-icon="inline-start" class={serviceLogsLoading ? 'animate-spin' : ''} />
						{text.refresh}
					</Button>
				</div>
				{#if serviceLogsError}
					<p class="text-sm text-destructive">{serviceLogsError}</p>
				{:else if serviceLogLines !== undefined && serviceLogLines.length === 0}
					<p class="text-sm text-muted-foreground">{text.serviceLogsEmpty}</p>
				{:else if serviceLogLines !== undefined}
					<RawDocument document={serviceLogLines.join('\n')} />
				{/if}
			</UnderlineTabs.Content>
		</UnderlineTabs.Root>
	{/if}
</main>
