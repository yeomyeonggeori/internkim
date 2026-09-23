<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import DecisionTable from './decision-table.svelte';
	import ExchangeView from './exchange-view.svelte';
	import { decisionMessageKeys, decisionRows, fetchLLMCallExchange, type Exchange, type LLMCallRecord } from './llm-calls';
	import { formatCostUSD } from './runs-api';
	import { formatLatency, formatTaskTimestamp } from './runs-view';
	import type { TasksText } from './text';

	let { llmCallID, record, createdAt, text }: { llmCallID?: string; record: LLMCallRecord; createdAt?: string; text: TasksText } = $props();

	let isOpen = $state(false);
	let exchange = $state<Exchange | undefined>(undefined);
	let exchangeError = $state('');
	let isLoading = $state(false);

	const messageKeys = $derived(decisionMessageKeys(record));
	const title = $derived(record.schemaName || record.kind);

	async function loadExchange() {
		if (!llmCallID || exchange || isLoading) return;
		isLoading = true;
		exchangeError = '';
		try {
			exchange = await fetchLLMCallExchange(llmCallID);
		} catch (error) {
			exchangeError = error instanceof Error && error.message ? error.message : text.exchangeLoadError;
		} finally {
			isLoading = false;
		}
	}

	function handleOpenChange(isNowOpen: boolean) {
		isOpen = isNowOpen;
		if (isNowOpen) void loadExchange();
	}

	function tokenSummary(): string {
		if (record.promptTokens === 0 && record.completionTokens === 0) return '';
		return `${record.promptTokens.toLocaleString()} → ${record.completionTokens.toLocaleString()}`;
	}
</script>

<article class="overflow-hidden rounded-lg border {record.isError ? 'border-destructive/40' : ''}">
	<div class="flex flex-col gap-2 px-3 py-3">
		<div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
			<Badge variant={record.isError ? 'destructive' : 'outline'}>{text.laneLLM}</Badge>
			<code class="min-w-0 truncate text-xs font-medium">{title}</code>
			{#if record.model}
				<span class="truncate text-xs text-muted-foreground">{record.model}{record.provider ? ` · ${record.provider}` : ''}</span>
			{/if}
			{#if record.usedFallback}
				<Badge variant="secondary" title={record.fallbackReason}>{text.fallbackUsed}</Badge>
			{/if}
			<span class="ml-auto flex shrink-0 items-center gap-3 text-xs text-muted-foreground tabular-nums">
				<span>{formatLatency(record.latencyMs)}</span>
				{#if tokenSummary()}<span>{tokenSummary()}</span>{/if}
				{#if record.costUSD > 0}<span class="text-foreground">{formatCostUSD(record.costUSD)}</span>{/if}
				<span>{formatTaskTimestamp(createdAt)}</span>
			</span>
		</div>
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
	</div>
	{#if llmCallID}
		<Collapsible.Root open={isOpen} onOpenChange={handleOpenChange} class="border-t">
			<Collapsible.Trigger class="group flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-muted-foreground hover:bg-muted/50">
				<ChevronRightIcon class="size-3.5 transition-transform group-data-[state=open]:rotate-90" />
				{text.modelSaw} · {text.modelAnswered}
			</Collapsible.Trigger>
			<Collapsible.Content class="border-t px-3 py-3">
				{#if isLoading}
					<Skeleton class="h-24 w-full" />
				{:else if exchangeError}
					<p class="text-xs text-destructive">{exchangeError}</p>
				{:else if exchange}
					<ExchangeView {exchange} {text} />
				{/if}
			</Collapsible.Content>
		</Collapsible.Root>
	{/if}
</article>
