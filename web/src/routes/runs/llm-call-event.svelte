<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import DecisionTable from './decision-table.svelte';
	import ExchangeView from './exchange-view.svelte';
	import { decisionMessageKeys, decisionRows, fetchLLMCallExchange, type Exchange, type LLMCallRecord } from './llm-calls';
	import { formatCostUSD } from './runs-api';
	import { formatLatency } from './runs-view';
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';

	let {
		value,
		isOpen,
		llmCallID,
		record,
		elapsed,
		text
	}: { value: string; isOpen: boolean; llmCallID?: string; record: LLMCallRecord; elapsed: string; text: TasksText } = $props();

	let exchangeRequest: Promise<Exchange> | undefined;

	const messageKeys = $derived(decisionMessageKeys(record));
	const meta = $derived([formatLatency(record.latencyMs), record.costUSD > 0 ? formatCostUSD(record.costUSD) : ''].filter(Boolean).join(' · '));
	const facts = $derived([record.model, record.provider, tokenSummary()].filter(Boolean).join(' · '));

	function tokenSummary(): string {
		if (record.promptTokens === 0 && record.completionTokens === 0) return '';
		return `${record.promptTokens.toLocaleString()} → ${record.completionTokens.toLocaleString()}`;
	}

	function exchangeOnce(llmCallID: string): Promise<Exchange> {
		exchangeRequest ??= fetchLLMCallExchange(llmCallID);
		return exchangeRequest;
	}

	function loadErrorOf(error: unknown): string {
		return error instanceof Error && error.message ? error.message : text.exchangeLoadError;
	}
</script>

<TimelineEvent
	{value}
	lane={record.isError ? 'failure' : 'llm'}
	laneLabel={record.isError ? text.laneFailure : text.laneLLM}
	title={record.schemaName || record.kind}
	{meta}
	{elapsed}
	{isOpen}
>
	{#if record.error}
		<p class="text-xs text-destructive">{record.error}</p>
	{/if}
	{#each messageKeys as messageKey (messageKey)}
		<div class="overflow-x-auto rounded-md border">
			{#if messageKeys.length > 1}
				<p class="border-b px-3 py-1.5 font-mono text-xs text-muted-foreground">{messageKey}</p>
			{/if}
			<DecisionTable rows={decisionRows(record, messageKey)} {text} />
		</div>
	{/each}
	{#if !llmCallID}
		<p class="text-xs text-muted-foreground">{facts}</p>
	{:else}
		{#await exchangeOnce(llmCallID)}
			<Skeleton class="h-24 w-full" />
		{:then exchange}
			<ExchangeView {exchange} callFacts={facts} {text} />
		{:catch error}
			<p class="text-xs text-destructive">{loadErrorOf(error)}</p>
		{/await}
	{/if}
</TimelineEvent>
