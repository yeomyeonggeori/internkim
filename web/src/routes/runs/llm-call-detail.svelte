<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import DecisionTable from './decision-table.svelte';
	import ExchangeView from './exchange-view.svelte';
	import { decisionMessageKeys, decisionRows, fetchLLMCallExchange, type Exchange, type LLMCallRecord } from './llm-calls';
	import type { TasksText } from './text';

	let { llmCallID, record, text }: { llmCallID?: string; record: LLMCallRecord; text: TasksText } = $props();

	let exchangeRequest: Promise<Exchange> | undefined;

	const messageKeys = $derived(decisionMessageKeys(record));
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
