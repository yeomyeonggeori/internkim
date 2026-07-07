<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import BotIcon from '@lucide/svelte/icons/bot';
	import ClipboardListIcon from '@lucide/svelte/icons/clipboard-list';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import { onMount } from 'svelte';
	import {
		eventLane,
		fetchServiceLogs,
		fetchTaskDetail,
		formatCostUSD,
		formatEventBody,
		summarizeTimeline,
		taskDetailShareText,
		taskEventShareText,
		type EventLane,
		type TaskDetail,
		type TaskEvent,
		type TimelineSummary
	} from '../tasks-api';
	import {
		eventLaneClass,
		formatLatency,
		formatTaskTimestamp,
		taskStatusBadgeVariant,
		taskStatusIcon,
		taskStatusLabel
	} from '../tasks-view';
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

	const summary = $derived(detail ? summarizeTimeline(detail.taskEvents) : undefined);
	const visibleTaskEvents = $derived(detail ? filterTaskEvents(detail.taskEvents) : []);
	const taskShareText = $derived(detail ? taskDetailShareText(detail) : '');
	const visibleEventsShareText = $derived(detail ? taskDetailShareText(detail, { events: visibleTaskEvents, title: 'Visible Task Events' }) : '');
	const eventLaneFilters = $derived(detail ? buildEventLaneFilters(detail.taskEvents) : []);
	const timelineSummaryRows = $derived(summary && detail ? buildTimelineSummaryRows(summary, detail.taskEvents.length) : []);
	const taskListPath = $derived(page.url.pathname.startsWith('/poc-admin') ? '/poc-admin' : '/tasks');

	async function load() {
		loadError = '';
		try {
			detail = await fetchTaskDetail(page.params.id ?? '');
		} catch {
			loadError = text.detailLoadError;
		}
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
			{ all: 0, llm: 0, tool: 0, failure: 0, control: 0 }
		);
		return [
			{ value: 'all', label: text.allEvents, count: counts.all },
			{ value: 'llm', label: text.laneLLM, count: counts.llm },
			{ value: 'tool', label: text.laneTool, count: counts.tool },
			{ value: 'failure', label: text.laneFailure, count: counts.failure },
			{ value: 'control', label: text.laneControl, count: counts.control }
		];
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
				return text.laneControl;
		}
	}

	function eventLaneBadgeVariant(lane: EventLane) {
		if (lane === 'failure') return 'destructive';
		if (lane === 'tool') return 'secondary';
		if (lane === 'llm') return 'outline';
		return 'ghost';
	}

	function taskEventPreview(taskEvent: TaskEvent): string {
		const compactBody = formatEventBody(taskEvent.body).replace(/\s+/g, ' ').trim();
		if (compactBody.length <= 180) return compactBody;
		return `${compactBody.slice(0, 180)}...`;
	}

	function buildTimelineSummaryRows(timelineSummary: TimelineSummary, eventCount: number) {
		return [
			{ label: text.eventCount, value: eventCount.toLocaleString() },
			{ label: text.llmCalls, value: timelineSummary.llmCallCount.toLocaleString() },
			{ label: text.llmLatency, value: formatLatency(timelineSummary.llmLatencyMS) },
			{ label: text.llmTokens, value: timelineSummary.llmTotalTokens.toLocaleString() },
			{ label: text.llmCost, value: formatCostUSD(timelineSummary.llmCostUSD) },
			{ label: text.toolCalls, value: timelineSummary.toolCallCount.toLocaleString() }
		];
	}

	onMount(load);
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="flex min-h-[calc(100svh-48px)] w-full self-start flex-col gap-5 px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<Button href={taskListPath} variant="ghost" size="sm">
			<ArrowLeftIcon data-icon="inline-start" />
			{text.backToList}
		</Button>
		{#if detail}
			<CopyButton text={taskShareText} variant="outline" size="sm">
				<span>{text.copyForAI}</span>
			</CopyButton>
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
		<section class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
			<Card.Root>
				<Card.Header class="gap-3">
					<div class="flex min-w-0 flex-wrap items-center gap-2">
						<Badge variant={taskStatusBadgeVariant(detail.taskRun.status)}>
							<StatusIcon />
							{taskStatusLabel(detail.taskRun.status, text)}
						</Badge>
						<code class="truncate rounded-md bg-muted px-2 py-1 text-xs">{detail.taskRun.taskRunID}</code>
					</div>
					<Card.Title class="text-lg">{text.taskSummaryTitle}</Card.Title>
					<Card.Description>{detail.taskRun.prompt || '—'}</Card.Description>
				</Card.Header>
				<Card.Content class="flex flex-col gap-4">
					<div class="grid gap-3 md:grid-cols-2">
						<div class="flex flex-col gap-1">
							<span class="text-xs text-muted-foreground">{text.createdAt}</span>
							<span class="text-sm">{formatTaskTimestamp(detail.taskRun.createdAt)}</span>
						</div>
						<div class="flex flex-col gap-1">
							<span class="text-xs text-muted-foreground">{text.updatedAt}</span>
							<span class="text-sm">{formatTaskTimestamp(detail.taskRun.updatedAt)}</span>
						</div>
						{#if detail.taskRun.requesterDisplayName || detail.taskRun.requesterPersonID}
							<div class="flex flex-col gap-1 md:col-span-2">
								<span class="text-xs text-muted-foreground">{text.requesterLabel}</span>
								<span class="text-sm">{detail.taskRun.requesterDisplayName || detail.taskRun.requesterPersonID}</span>
							</div>
						{/if}
					</div>
					{#if detail.taskRun.failureReason}
						<Separator />
						<div class="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
							<span class="font-medium">{text.failureReasonLabel}:</span>
							{detail.taskRun.failureReason}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			{#if summary}
				<Card.Root size="sm">
					<Card.Content class="px-0 py-0">
						<Table.Root>
							<Table.Body>
								{#each timelineSummaryRows as row (row.label)}
									<Table.Row>
										<Table.Cell class="h-9 py-0 text-xs text-muted-foreground">{row.label}</Table.Cell>
										<Table.Cell class="h-9 py-0 text-right text-sm font-medium tabular-nums">{row.value}</Table.Cell>
									</Table.Row>
								{/each}
							</Table.Body>
						</Table.Root>
					</Card.Content>
				</Card.Root>
			{/if}
		</section>

		<Tabs.Root bind:value={selectedTab} class="min-w-0">
			<Tabs.List>
				<Tabs.Trigger value="timeline">
					<ClipboardListIcon data-icon="inline-start" />
					{text.timelineTab}
				</Tabs.Trigger>
				<Tabs.Trigger value="brief">
					<FileTextIcon data-icon="inline-start" />
					{text.briefTab}
				</Tabs.Trigger>
				<Tabs.Trigger value="logs">
					<TerminalIcon data-icon="inline-start" />
					{text.logsTab}
				</Tabs.Trigger>
			</Tabs.List>

			<Tabs.Content value="timeline" class="min-w-0">
				<Card.Root>
					<Card.Header class="gap-3">
						<div class="flex min-w-0 flex-wrap items-start justify-between gap-3">
							<div>
								<Card.Title>{text.timelineTitle}</Card.Title>
								<Card.Description>
									{text.visibleEvents.replace('{count}', String(visibleTaskEvents.length)).replace('{total}', String(detail.taskEvents.length))}
								</Card.Description>
							</div>
							<CopyButton text={visibleEventsShareText} variant="outline" size="sm" disabled={visibleTaskEvents.length === 0}>
								<span>{text.copyVisibleEvents}</span>
							</CopyButton>
						</div>
						<div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
							<Tabs.Root bind:value={selectedEventLane} class="min-w-0">
								<Tabs.List variant="line" class="max-w-full overflow-x-auto">
									{#each eventLaneFilters as filter (filter.value)}
										<Tabs.Trigger value={filter.value} class="gap-1">
											{filter.label}
											<Badge variant="secondary" class="h-4 px-1.5 text-[10px]">{filter.count}</Badge>
										</Tabs.Trigger>
									{/each}
								</Tabs.List>
							</Tabs.Root>
							<label class="relative min-w-0 lg:w-80">
								<SearchIcon class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
								<Input bind:value={eventSearchQuery} placeholder={text.searchEvents} class="pl-8" />
							</label>
						</div>
					</Card.Header>
					<Card.Content>
						{#if visibleTaskEvents.length === 0}
							<div class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
								{text.noMatchingEvents}
							</div>
						{:else}
							<div class="flex flex-col gap-3">
								{#each visibleTaskEvents as taskEvent, index (`${taskEvent.name}-${taskEvent.createdAt ?? 'event'}-${index}`)}
									{@const lane = eventLane(taskEvent.name)}
									<article class={`overflow-hidden rounded-lg border ${eventLaneClass(lane)}`}>
										<div class="flex flex-col gap-2 px-3 py-3">
											<div class="flex min-w-0 flex-wrap items-center gap-2">
												<Badge variant={eventLaneBadgeVariant(lane)}>{eventLaneLabel(lane)}</Badge>
												<code class="min-w-0 flex-1 truncate text-xs">{taskEvent.name}</code>
												<span class="text-xs whitespace-nowrap text-muted-foreground">{formatTaskTimestamp(taskEvent.createdAt)}</span>
												<CopyButton text={taskEventShareText(taskEvent, index + 1)} variant="ghost" size="sm">
													<span>{text.copyEvent}</span>
												</CopyButton>
											</div>
											<p class="line-clamp-2 text-xs break-words text-muted-foreground">{taskEventPreview(taskEvent)}</p>
										</div>
										<pre class="max-h-80 overflow-auto border-t bg-background/70 px-3 py-3 text-xs leading-relaxed whitespace-pre-wrap">{formatEventBody(taskEvent.body)}</pre>
									</article>
								{/each}
							</div>
						{/if}
					</Card.Content>
				</Card.Root>
			</Tabs.Content>

			<Tabs.Content value="brief">
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
			</Tabs.Content>

			<Tabs.Content value="logs">
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
			</Tabs.Content>
		</Tabs.Root>
	{/if}
</main>
