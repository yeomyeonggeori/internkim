<script lang="ts">
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import ColorMarker from '$lib/components/color-marker.svelte';
	import * as Field from '$lib/components/ui/field';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { Spinner } from '$lib/components/ui/spinner';
	import { fetchMemoryRecall, type MemoryLayer, type MemoryRecalled } from './memory-facts-api';
	import { memoryLayerTones } from './memory-workbench-model';
	import type { MemoryText } from './text';

	type Props = { labelOf: (layer: MemoryLayer) => string; onSelectFact: (factID: string) => void; text: MemoryText };

	let { labelOf, onSelectFact, text }: Props = $props();
	let question = $state('');
	let recalled = $state<MemoryRecalled[] | null>(null);
	let isRecalling = $state(false);
	let hasFailed = $state(false);
	let requestSequence = 0;

	async function recall(): Promise<void> {
		const trimmed = question.trim();
		if (!trimmed) return;
		const requestID = ++requestSequence;
		isRecalling = true;
		hasFailed = false;
		try {
			const answer = await fetchMemoryRecall(trimmed);
			if (requestID === requestSequence) recalled = answer.facts;
		} catch {
			if (requestID === requestSequence) hasFailed = true;
		} finally {
			if (requestID === requestSequence) isRecalling = false;
		}
	}
</script>

<section class="flex flex-col gap-3 rounded-lg border bg-muted/30 p-3" aria-labelledby="memory-recall-title">
	<form role="search" class="flex flex-col gap-2" onsubmit={(event) => { event.preventDefault(); void recall(); }}>
		<Field.Field>
			<Field.Label id="memory-recall-title" for="memory-recall-question">{text.recallTitle}</Field.Label>
			<InputGroup.Root>
				<InputGroup.Input id="memory-recall-question" bind:value={question} placeholder={text.recallPlaceholder} />
				<InputGroup.Addon align="inline-end">
					<InputGroup.Button type="submit" size="icon-xs" disabled={isRecalling || !question.trim()} aria-label={text.recallAction}>
						{#if isRecalling}<Spinner />{:else}<SparklesIcon />{/if}
					</InputGroup.Button>
				</InputGroup.Addon>
			</InputGroup.Root>
		</Field.Field>
	</form>
	{#if hasFailed}
		<p role="alert" class="text-xs text-destructive">{text.recallFailed}</p>
	{:else if recalled}
		{#if recalled.length > 0}
			<ol class="flex flex-col gap-1" aria-label={text.recallTitle}>
				{#each recalled as fact (fact.factID)}
					<li>
						<button type="button" class="flex w-full min-w-0 items-start gap-2 rounded-md px-1.5 py-1 text-left text-sm hover:bg-muted/60 focus-visible:outline-2 focus-visible:outline-ring" onclick={() => onSelectFact(fact.factID)}>
							<ColorMarker class={`mt-1 ${memoryLayerTones[fact.scopeType]}`} />
							<span class="flex min-w-0 flex-col"><span class="break-words">{fact.content}</span><span class="text-xs text-muted-foreground">{labelOf(fact)}</span></span>
						</button>
					</li>
				{/each}
			</ol>
		{:else}
			<p class="text-xs text-muted-foreground">{text.recallEmpty}</p>
		{/if}
	{/if}
	<p class="text-xs text-muted-foreground">{text.recallDescription}</p>
</section>
