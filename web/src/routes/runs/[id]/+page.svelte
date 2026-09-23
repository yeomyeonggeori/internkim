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
	import BotIcon from '@lucide/svelte/icons/bot';
	import ClipboardListIcon from '@lucide/svelte/icons/clipboard-list';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import { onDestroy } from 'svelte';
	import ApprovalDecision from '../approval-decision.svelte';
	import RetryTaskButton from '../retry-task-button.svelte';
	import LLMCallEvent from '../llm-call-event.svelte';
	import RawDocument from '../raw-document.svelte';
	import TimelineEvent from '../timeline-event.svelte';
	import TurnInputEvent from '../turn-input-event.svelte';
	import { readLLMCallRecord } from '../llm-calls';
	import {
		eventLane,
		fetchServiceLogs,
		fetchTaskDetail,
		formatCostUSD,
		formatEventBody,
		pendingApprovalOf,
		summarizeTimeline,
		taskDetailShareText,
		type EventLane,
		type TaskDetail,
		type TaskEvent,
		type TimelineSummary
	} from '../runs-api';
	import {
		formatElapsed,
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
	let selectedTab = $state('timeline');
	let selectedEventLane = $state('all');
	let eventSearchQuery = $state('');
	let serviceLogLines = $state<string[] | undefined>(undefined);
	let serviceLogsLoading = $state(false);
	let serviceLogsError = $state('');
	let pollTimer: ReturnType<typeof setTimeout> | undefined;
	let loadGeneration = 0;

	const summary = $derived(detail ? summarizeTimeline(detail.taskEvents) : undefined);
	const pendingApproval = $derived(detail ? pendingApprovalOf(detail) : undefined);
	const visibleTaskEvents = $derived(detail ? filterTaskEvents(detail.taskEvents) : []);
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

	function filterTaskEvents(taskEvents: TaskEvent[]): TaskEvent[] {
		const query = eventSearchQuery.trim().toLowerCase();
		return taskEvents.filter((taskEvent) => {
			const lane = eventLane(taskEvent.name);
			if (selectedEventLane !== 'all' && selectedEventLane !== lane) return false;
			if (!query) return true;
			return `${taskEvent.name}\n${formatEventBody(taskEvent.body)}`.toLowerCase().includes(query);
		});
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

	let openEventValues = $state<string[]>([]);

	function summaryLine(timelineSummary: TimelineSummary): string {
		return text.summaryLine
			.replace('{calls}', timelineSummary.llmCallCount.toLocaleString())
			.replace('{latency}', formatLatency(timelineSummary.llmLatencyMS))
			.replace('{tokens}', timelineSummary.llmTotalTokens.toLocaleString())
			.replace('{cost}', formatCostUSD(timelineSummary.llmCostUSD))
			.replace('{tools}', timelineSummary.toolCallCount.toLocaleString());
	}

	function eventLaneLabel(lane: EventLane): string {
		switch (lane) {
			case 'llm':
				return text.laneLLM;
			case 'tool':
				return text.laneTool;
			case 'failure':
				return text.laneFailure;
			default:
				return text.laneOther;
		}
	}




	$effect(() => {
		const taskRunID = page.params.id ?? '';
		loadGeneration += 1;
		const generation = loadGeneration;
		detail = undefined;
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
			</div>
			<h1 class="line-clamp-3 text-lg leading-snug font-semibold">{detail.taskRun.prompt || detail.taskRun.taskRunID}</h1>
			{#if summary}
				<p class="text-xs text-muted-foreground tabular-nums">{summaryLine(summary)}</p>
			{/if}
			{#if detail.taskRun.failureReason}
				<p class="text-sm text-destructive">{detail.taskRun.failureReason}</p>
			{/if}
			{#if pendingApproval}
				<ApprovalDecision approval={pendingApproval} {text} onDecided={() => void load(page.params.id ?? '', loadGeneration)} />
			{/if}
		</section>

		<UnderlineTabs.Root bind:value={selectedTab} class="min-w-0">
			<UnderlineTabs.List>
				<UnderlineTabs.Trigger value="timeline">
					<ClipboardListIcon data-icon="inline-start" />
					{text.timelineTab}
				</UnderlineTabs.Trigger>
				<UnderlineTabs.Trigger value="brief">
					<FileTextIcon data-icon="inline-start" />
					{text.briefTab}
				</UnderlineTabs.Trigger>
				<UnderlineTabs.Trigger value="logs">
					<TerminalIcon data-icon="inline-start" />
					{text.logsTab}
				</UnderlineTabs.Trigger>
			</UnderlineTabs.List>

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
				{#if visibleTaskEvents.length === 0}
					<p class="py-8 text-center text-sm text-muted-foreground">{text.noMatchingEvents}</p>
				{:else}
					<Accordion.Root type="multiple" bind:value={openEventValues} class="border-t">
						{#each visibleTaskEvents as taskEvent, index (`${taskEvent.name}-${taskEvent.createdAt ?? 'event'}-${index}`)}
							{@const lane = eventLane(taskEvent.name)}
							{@const llmCallRecord = lane === 'llm' ? readLLMCallRecord(taskEvent.body) : undefined}
							{@const value = taskEvent.id ?? `${taskEvent.name}-${taskEvent.createdAt ?? 'event'}-${index}`}
							{@const isOpen = openEventValues.includes(value)}
							{@const elapsed = formatElapsed(detail.taskRun.createdAt, taskEvent.createdAt)}
							{#if llmCallRecord}
								<LLMCallEvent {value} {isOpen} {elapsed} llmCallID={taskEvent.id} record={llmCallRecord} {text} />
							{:else if taskEvent.name === 'task.turn_input' && taskEvent.id}
								<TurnInputEvent {value} {isOpen} {elapsed} taskEventID={taskEvent.id} {text} />
							{:else}
								<TimelineEvent {value} {isOpen} {elapsed} {lane} laneLabel={eventLaneLabel(lane)} title={taskEvent.name}>
									<RawDocument document={formatEventBody(taskEvent.body)} />
								</TimelineEvent>
							{/if}
						{/each}
					</Accordion.Root>
				{/if}
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="brief">
				<div class="grid gap-4 lg:grid-cols-2">
					<Card.Root>
						<Card.Header>
							<Card.Title>{text.promptLabel}</Card.Title>
							<Card.Action>
								<CopyButton text={detail.taskRun.prompt ?? ''} variant="ghost" size="sm" disabled={!detail.taskRun.prompt}>
									<span>{text.copyPrompt}</span>
								</CopyButton>
							</Card.Action>
						</Card.Header>
						<Card.Content>
							<p class="whitespace-pre-wrap text-sm">{detail.taskRun.prompt || '—'}</p>
						</Card.Content>
					</Card.Root>

					<Card.Root>
						<Card.Header>
							<Card.Title>{text.resultLabel}</Card.Title>
							<Card.Action>
								<CopyButton text={detail.taskRun.result ?? ''} variant="ghost" size="sm" disabled={!detail.taskRun.result}>
									<span>{text.copyResult}</span>
								</CopyButton>
							</Card.Action>
						</Card.Header>
						<Card.Content>
							<p class="whitespace-pre-wrap text-sm">{detail.taskRun.result || '—'}</p>
						</Card.Content>
					</Card.Root>
				</div>
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="logs">
				<Card.Root>
					<Card.Header>
						<Card.Title>{text.serviceLogsTitle}</Card.Title>
						<Card.Description>{text.serviceLogsDescription}</Card.Description>
						<Card.Action>
							<Button onclick={loadServiceLogs} disabled={serviceLogsLoading} variant="outline" size="sm">
								<RefreshCwIcon data-icon="inline-start" class={serviceLogsLoading ? 'animate-spin' : ''} />
								{text.serviceLogsLoad}
							</Button>
						</Card.Action>
					</Card.Header>
					<Card.Content>
						{#if serviceLogsError}
							<p class="text-sm text-destructive">{serviceLogsError}</p>
						{:else if serviceLogLines === undefined}
							<div class="flex items-center gap-2 text-sm text-muted-foreground">
								<BotIcon />
								{text.serviceLogsPrompt}
							</div>
						{:else if serviceLogLines.length === 0}
							<p class="text-sm text-muted-foreground">{text.serviceLogsEmpty}</p>
						{:else}
							<div class="flex flex-col gap-2">
								<CopyButton text={serviceLogLines.join('\n')} variant="outline" size="sm" class="w-fit">
									<span>{text.copyLogs}</span>
								</CopyButton>
								<pre class="max-h-96 overflow-auto rounded-lg border bg-muted/30 px-3 py-3 text-xs leading-relaxed whitespace-pre-wrap">{serviceLogLines.join('\n')}</pre>
							</div>
						{/if}
					</Card.Content>
				</Card.Root>
			</UnderlineTabs.Content>
		</UnderlineTabs.Root>
	{/if}
</main>
