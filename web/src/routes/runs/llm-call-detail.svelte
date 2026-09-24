<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import DecisionTable from './decision-table.svelte';
	import ExchangeView from './exchange-view.svelte';
	import FactList, { type Fact } from './fact-list.svelte';
	import { decisionMessageKeys, decisionRows, fetchLLMCallExchange, type Exchange, type LLMCallRecord } from './llm-calls';
	import type { TasksText } from './text';

	let { llmCallID, record, text, isModelInHeader = false }: { llmCallID?: string; record: LLMCallRecord; text: TasksText; isModelInHeader?: boolean } = $props();

	let exchangeRequest: Promise<Exchange> | undefined;

	const messageKeys = $derived(decisionMessageKeys(record));
	const facts = $derived(callFacts());

	function callFacts(): Fact[] {
		const facts: Fact[] = [];
		if (record.model && !isModelInHeader) facts.push({ label: text.factModel, value: record.model, isIdentifier: true });
		if (record.provider) facts.push({ label: text.factProvider, value: record.provider, isIdentifier: true });
		if (record.promptTokens > 0 || record.completionTokens > 0) {
			facts.push({ label: text.factTokens, value: `${record.promptTokens.toLocaleString()} → ${record.completionTokens.toLocaleString()}` });
		}
		return facts;
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
{#snippet decisionTables()}
	{#each messageKeys as messageKey (messageKey)}
		<div class="overflow-x-auto rounded-md border">
			{#if messageKeys.length > 1}
				<p class="border-b px-3 py-1.5 font-mono text-xs text-muted-foreground">{messageKey}</p>
			{/if}
			<DecisionTable rows={decisionRows(record, messageKey)} {text} />
		</div>
	{/each}
{/snippet}

{#if !llmCallID}
	<FactList {facts} />
	{@render decisionTables()}
{:else}
	{#await exchangeOnce(llmCallID)}
		<Skeleton class="h-24 w-full" />
	{:then exchange}
		<ExchangeView {exchange} callFacts={facts} {text}>
			{@render decisionTables()}
		</ExchangeView>
	{:catch error}
		<p class="text-xs text-destructive">{loadErrorOf(error)}</p>
	{/await}
{/if}
