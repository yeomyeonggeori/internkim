<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import { onMount } from 'svelte';
	import {
		deleteMemoryEpisode,
		deletePinnedMemory,
		fetchMemoryGraph,
		type MemoryGraphFact,
		type MemoryGraphResponse
	} from './memory-graph-api';
	import {
		allFilterValue,
		emptyMemoryFactFilters,
		episodeForFact,
		filterMemoryFacts,
		memoryFactScopes,
		memoryFactSourceKinds,
		personalScopeFilterValue,
		sortMemoryFactsByRecency
	} from './memory-fact-list-model';
	import { isPersonalScope } from './memory-graph-selection';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	let memoryGraph = $state<MemoryGraphResponse | null>(null);
	let filters = $state(emptyMemoryFactFilters());
	let errorMessage = $state('');
	let actionErrorMessage = $state('');
	let isLoading = $state(false);

	const facts = $derived(memoryGraph?.facts ?? []);
	const episodes = $derived(memoryGraph?.episodes ?? []);
	const visibleFacts = $derived(sortMemoryFactsByRecency(filterMemoryFacts(facts, filters)));
	const hasActiveFilters = $derived(
		filters.searchText.trim() !== '' || filters.sourceKind !== allFilterValue || filters.scope !== allFilterValue
	);

	const sourceKindOptions = $derived([
		{ value: allFilterValue, label: text.factFilterKindAll },
		...memoryFactSourceKinds(facts).map((sourceKind) => ({ value: sourceKind, label: sourceKindLabel(sourceKind) }))
	]);
	const scopeOptions = $derived([
		{ value: allFilterValue, label: text.factFilterScopeAll },
		...memoryFactScopes(facts).map((scope) => ({ value: scope, label: scopeLabel(scope) }))
	]);

	onMount(loadMemoryGraph);

	async function loadMemoryGraph(): Promise<void> {
		isLoading = true;
		errorMessage = '';
		actionErrorMessage = '';
		try {
			memoryGraph = await fetchMemoryGraph('');
		} catch {
			errorMessage = text.loadFailed;
		} finally {
			isLoading = false;
		}
	}

	function resetFilters(): void {
		filters = emptyMemoryFactFilters();
	}

	function sourceKindLabel(sourceKind: string): string {
		if (sourceKind === 'fact') return text.factKindFact;
		if (sourceKind === 'node') return text.factKindNode;
		if (sourceKind === 'episode') return text.factKindEpisode;
		if (sourceKind === 'pinned') return text.factKindPinned;
		return sourceKind;
	}

	function scopeLabel(scope: string): string {
		if (scope === personalScopeFilterValue) return text.factScopePersonal;
		if (scope === 'circle') return text.factScopeCircle;
		if (scope === 'workspace') return text.factScopeWorkspace;
		if (scope === 'conversation') return text.factScopeConversation;
		return scope;
	}

	function scopeDisplayName(fact: MemoryGraphFact): string {
		return isPersonalScope(fact.scopeType) ? text.myMemory : fact.namespaceID;
	}

	function validAtText(fact: MemoryGraphFact): string {
		return fact.validAt ? fact.validAt.slice(0, 10) : text.validAtUnavailable;
	}

	function factScoreText(score: number | null | undefined): string {
		if (typeof score !== 'number' || !Number.isFinite(score)) return text.scoreUnavailable;
		return `${text.score} ${Math.round(score * 100)}%`;
	}

	function countSummaryText(): string {
		return text.factListCountTemplate
			.replace('{visible}', String(visibleFacts.length))
			.replace('{total}', String(facts.length));
	}

	function canDeleteFact(fact: MemoryGraphFact): boolean {
		return fact.sourceKind === 'pinned' || Boolean(episodeForFact(episodes, fact));
	}

	function confirmDeleteFact(fact: MemoryGraphFact): void {
		const isPinned = fact.sourceKind === 'pinned';
		confirmDelete({
			title: text.memoryDeleteTitle,
			description: isPinned ? text.pinnedMemoryDeleteDescription : text.factDeleteDescription,
			confirm: { text: text.memoryDeleteAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await deleteFact(fact);
			}
		});
	}

	async function deleteFact(fact: MemoryGraphFact): Promise<void> {
		actionErrorMessage = '';
		try {
			if (fact.sourceKind === 'pinned') {
				await deletePinnedMemory();
			} else {
				const episode = episodeForFact(episodes, fact);
				if (!episode) return;
				await deleteMemoryEpisode(episode.episodeID, episode.namespaceIDs ?? []);
			}
			await loadMemoryGraph();
		} catch {
			actionErrorMessage = text.memoryDeleteFailed;
		}
	}
</script>

<section class="grid min-w-0 gap-3">
	<div class="flex min-w-0 flex-wrap items-center gap-2">
		<div class="relative min-w-48 flex-1">
			<SearchIcon class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
			<Input bind:value={filters.searchText} placeholder={text.factFilterPlaceholder} autocomplete="off" class="pl-8" />
		</div>
		{@render FilterSelect(sourceKindOptions, filters.sourceKind, (value: string) => (filters.sourceKind = value))}
		{@render FilterSelect(scopeOptions, filters.scope, (value: string) => (filters.scope = value))}
		{#if hasActiveFilters}
			<Button type="button" variant="ghost" size="sm" onclick={resetFilters} class="gap-2">
				<RotateCcwIcon class="size-4" />
				{text.factFilterReset}
			</Button>
		{/if}
		<Button type="button" variant="outline" size="sm" disabled={isLoading} onclick={loadMemoryGraph} class="gap-2">
			{#if isLoading}
				<LoaderIcon class="size-4 animate-spin" />
			{:else}
				<RefreshCwIcon class="size-4" />
			{/if}
			{text.refresh}
		</Button>
	</div>

	<p class="text-xs text-muted-foreground">{countSummaryText()}</p>

	{#if errorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
	{/if}
	{#if actionErrorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{actionErrorMessage}</p>
	{/if}

	{#if visibleFacts.length === 0 && !isLoading}
		<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">
			{facts.length === 0 ? text.noVisibleMemory : text.factListEmpty}
		</p>
	{:else}
		<div class="overflow-hidden rounded-lg border">
			{#each visibleFacts as fact (`${fact.namespaceID}:${fact.factID}`)}
				<article class="grid gap-2 border-b px-3 py-3 last:border-b-0">
					<div class="flex min-w-0 flex-wrap items-center gap-2">
						<Badge variant="outline">{sourceKindLabel(fact.sourceKind ?? '') || text.source}</Badge>
						<span class="truncate text-xs text-muted-foreground">{scopeDisplayName(fact)}</span>
						<span class="text-xs tabular-nums text-muted-foreground">{validAtText(fact)}</span>
						<span class="ml-auto text-xs tabular-nums text-muted-foreground">{factScoreText(fact.score)}</span>
						{#if canDeleteFact(fact)}
							<Button
								type="button"
								variant="ghost"
								size="icon-sm"
								onclick={() => confirmDeleteFact(fact)}
								aria-label={text.memoryDelete}
								title={text.memoryDelete}
							>
								<TrashIcon class="size-4" />
							</Button>
						{/if}
					</div>
					<div class="memory-fact-markdown text-sm leading-5">
						<SvelteMarkdown source={fact.content} />
					</div>
				</article>
			{/each}
		</div>
	{/if}
</section>

{#snippet FilterSelect(options: { value: string; label: string }[], value: string, onchange: (value: string) => void)}
	<Select.Root type="single" {value} onValueChange={onchange}>
		<Select.Trigger class="w-36" size="sm">
			{options.find((option) => option.value === value)?.label ?? '-'}
		</Select.Trigger>
		<Select.Content>
			{#each options as option (option.value)}
				<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/snippet}

<style>
	.memory-fact-markdown :global(h1) {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0.25rem 0;
	}
	.memory-fact-markdown :global(h2),
	.memory-fact-markdown :global(h3) {
		font-size: 0.8125rem;
		font-weight: 600;
		margin: 0.5rem 0 0.25rem;
		color: var(--color-muted-foreground);
	}
	.memory-fact-markdown :global(p) {
		margin: 0.25rem 0;
	}
	.memory-fact-markdown :global(ul),
	.memory-fact-markdown :global(ol) {
		margin: 0.25rem 0;
		padding-left: 1.1rem;
	}
	.memory-fact-markdown :global(ul) {
		list-style: disc;
	}
	.memory-fact-markdown :global(ol) {
		list-style: decimal;
	}
	.memory-fact-markdown :global(li) {
		margin: 0.125rem 0;
	}
	.memory-fact-markdown :global(strong) {
		font-weight: 600;
	}
	.memory-fact-markdown :global(code) {
		font-family: var(--font-mono, monospace);
		font-size: 0.8125rem;
		background: var(--color-muted);
		padding: 0.05rem 0.25rem;
		border-radius: 0.25rem;
	}
</style>
