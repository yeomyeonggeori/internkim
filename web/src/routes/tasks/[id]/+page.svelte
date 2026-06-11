<script lang="ts">
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import { page } from '$app/state';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import {
		eventLane,
		fetchServiceLogs,
		fetchTaskDetail,
		formatEventBody,
		summarizeTimeline,
		type TaskDetail
	} from '../tasks-api';
	import {
		eventLaneClass,
		formatLatency,
		formatTaskTimestamp,
		taskStatusBadgeClass
	} from '../tasks-view';
	import { tasksText } from '../text';

	const text = createPageText(tasksText);
	let detail = $state<TaskDetail | undefined>(undefined);
	let loadError = $state('');

	const summary = $derived(detail ? summarizeTimeline(detail.taskEvents) : undefined);
	const laneLabels = $derived<Record<string, string>>({
		llm: text.laneLLM,
		tool: text.laneTool,
		failure: text.laneFailure,
		control: text.laneControl
	});

	let serviceLogLines = $state<string[] | undefined>(undefined);
	let serviceLogsLoading = $state(false);
	let serviceLogsError = $state('');

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

	onMount(load);
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="grid min-h-[calc(100svh-48px)] w-full flex-1 content-start gap-5 overflow-x-hidden px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<a href="/tasks" class="inline-flex w-fit items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
		<ArrowLeftIcon class="size-3.5" />
		{text.backToList}
	</a>

	{#if loadError}
		<p class="text-sm text-red-600">{loadError}</p>
	{:else if detail}
		<section class="grid min-w-0 gap-2">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<code class="text-sm">{detail.taskRun.taskRunID}</code>
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium {taskStatusBadgeClass(detail.taskRun.status)}">
					{detail.taskRun.status}
				</span>
				<span class="ml-auto text-xs text-muted-foreground">
					{text.createdAt} {formatTaskTimestamp(detail.taskRun.createdAt)} · {text.updatedAt}
					{formatTaskTimestamp(detail.taskRun.updatedAt)}
				</span>
			</div>
			<div class="grid gap-1 rounded-lg border px-4 py-3 text-sm">
				<p class="min-w-0"><span class="text-muted-foreground">{text.promptLabel}:</span> {detail.taskRun.prompt || '—'}</p>
				{#if detail.taskRun.result}
					<p class="min-w-0"><span class="text-muted-foreground">{text.resultLabel}:</span> {detail.taskRun.result}</p>
				{/if}
				{#if detail.taskRun.failureReason}
					<p class="min-w-0 text-red-600">
						<span class="text-muted-foreground">{text.failureReasonLabel}:</span>
						{detail.taskRun.failureReason}
					</p>
				{/if}
			</div>
			{#if summary}
				<div class="flex flex-wrap gap-4 text-xs text-muted-foreground">
					<span>{text.eventCount} {detail.taskEvents.length}</span>
					<span>{text.llmCalls} {summary.llmCallCount}</span>
					<span>{text.llmLatency} {formatLatency(summary.llmLatencyMS)}</span>
					{#if summary.llmTotalTokens > 0}
						<span>{text.llmTokens} {summary.llmTotalTokens.toLocaleString()}</span>
					{/if}
					<span>{text.toolCalls} {summary.toolCallCount}</span>
				</div>
			{/if}
		</section>

		<section class="grid min-w-0 gap-1.5">
			<h2 class="text-sm font-semibold">{text.timelineTitle}</h2>
			{#each detail.taskEvents as taskEvent, index (index)}
				{@const lane = eventLane(taskEvent.name)}
				<details class="min-w-0 rounded-md border border-l-4 {eventLaneClass(lane)}">
					<summary class="flex min-w-0 cursor-pointer flex-wrap items-center gap-2 px-3 py-1.5 text-xs">
						<span class="w-12 shrink-0 text-[10px] uppercase tracking-wide text-muted-foreground">
							{laneLabels[lane]}
						</span>
						<code class="truncate">{taskEvent.name}</code>
						<span class="ml-auto shrink-0 text-muted-foreground">
							{formatTaskTimestamp(taskEvent.createdAt)}
						</span>
					</summary>
					<pre class="overflow-x-auto border-t bg-muted/30 px-3 py-2 text-[11px] leading-relaxed">{formatEventBody(taskEvent.body)}</pre>
				</details>
			{/each}
		</section>

		<section class="grid min-w-0 gap-1.5">
			<h2 class="text-sm font-semibold">{text.serviceLogsTitle}</h2>
			<div>
				<button
					onclick={loadServiceLogs}
					disabled={serviceLogsLoading}
					class="rounded-md border px-3 py-1.5 text-xs disabled:opacity-50"
				>
					{text.serviceLogsLoad}
				</button>
			</div>
			{#if serviceLogsError}
				<p class="text-sm text-red-600">{serviceLogsError}</p>
			{:else if serviceLogLines !== undefined}
				{#if serviceLogLines.length === 0}
					<p class="text-sm text-muted-foreground">{text.serviceLogsEmpty}</p>
				{:else}
					<pre class="overflow-x-auto rounded-md border bg-muted/30 px-3 py-2 text-[11px] leading-relaxed">{serviceLogLines.join('\n')}</pre>
				{/if}
			{/if}
		</section>
	{/if}
</main>
