<script lang="ts">
	import { onMount } from 'svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import RefreshIcon from '@lucide/svelte/icons/refresh-cw';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Empty from '$lib/components/ui/empty';
	import { cn } from '$lib/utils';
	import MemoryFactDetail from './memory-fact-detail.svelte';
	import { fetchMemoryGraph, type MemoryGraphResponse } from './memory-graph-api';
	import { memoryFactKey, isCurrentMemoryFact, memoryAudience, memoryDate } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { text, mode = 'list' }: { text: MemoryText; mode?: 'list' | 'search' } = $props();
	let memoryGraph = $state<MemoryGraphResponse | null>(null);
	let selectedKey = $state('');
	let query = $state('');
	let submittedQuery = $state('');
	let isLoading = $state(false);
	let hasSearched = $state(false);
	let includesPrevious = $state(false);
	let errorMessage = $state('');
	let requestSequence = 0;
	const rememberedFacts = $derived((memoryGraph?.facts ?? []).filter((fact) => fact.sourceKind === 'fact'));
	const visibleFacts = $derived(rememberedFacts.filter((fact) => includesPrevious || isCurrentMemoryFact(fact)));
	const selectedFact = $derived(visibleFacts.find((fact) => memoryFactKey(fact) === selectedKey));
	const isUnavailable = $derived(memoryGraph?.health?.configured === false || memoryGraph?.health?.reachable === false);
	const isIncomplete = $derived(memoryGraph?.retrieval?.complete === false);

	onMount(() => { if (mode === 'list') void loadMemory(''); });

	async function loadMemory(searchQuery: string): Promise<void> {
		const requestID = ++requestSequence;
		isLoading = true;
		errorMessage = '';
		try {
			const response = await fetchMemoryGraph(searchQuery);
			if (requestID !== requestSequence) return;
			memoryGraph = response;
			submittedQuery = searchQuery;
		} catch {
			if (requestID === requestSequence) errorMessage = text.loadFailed;
		} finally {
			if (requestID === requestSequence) isLoading = false;
		}
	}

	function searchMemory(event: SubmitEvent): void {
		event.preventDefault();
		if (!query.trim() || isLoading) return;
		hasSearched = true;
		selectedKey = '';
		memoryGraph = null;
		void loadMemory(query.trim());
	}

	async function refreshMemory(): Promise<void> {
		await loadMemory(mode === 'search' ? submittedQuery : '');
	}
</script>

{#if mode === 'search'}
	<form class="flex items-end gap-2" onsubmit={searchMemory}>
		<div class="grid min-w-0 flex-1 gap-2">
			<label for="memory-search" class="text-sm font-medium">{text.searchPrompt}</label>
			<Input id="memory-search" bind:value={query} placeholder={text.searchPlaceholder} />
		</div>
		<Button type="submit" disabled={isLoading || !query.trim()}><SearchIcon data-icon="inline-start" />{text.search}</Button>
	</form>
{/if}

{#if mode === 'search' && !hasSearched}
	<Empty.Root class="min-h-80">
		<Empty.Header>
			<Empty.Media variant="icon"><SearchIcon /></Empty.Media>
			<Empty.Title>{text.searchTab}</Empty.Title>
			<Empty.Description>{text.searchDescription}</Empty.Description>
		</Empty.Header>
	</Empty.Root>
{:else}
	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="text-sm text-muted-foreground" aria-live="polite">{mode === 'search' ? submittedQuery : text.currentMemories}</p>
		<div class="flex items-center gap-2">
			{#if mode === 'list'}
				<Button variant="ghost" size="sm" aria-pressed={includesPrevious} onclick={() => includesPrevious = !includesPrevious}>
					{includesPrevious ? text.hidePrevious : text.showPrevious}
				</Button>
			{/if}
			<Button variant="ghost" size="icon-sm" disabled={isLoading} onclick={refreshMemory} aria-label={text.refresh}><RefreshIcon /></Button>
		</div>
	</div>
	{#if errorMessage || isUnavailable || isIncomplete}
		<div class="flex flex-wrap items-center justify-between gap-3 border-y py-3" role="status">
			<p class="max-w-2xl text-sm">{errorMessage || (isUnavailable ? text.unreachable : text.incompleteResults)}</p>
			{#if errorMessage}<Button variant="outline" size="sm" onclick={refreshMemory}>{text.refresh}</Button>{/if}
		</div>
	{/if}
	{#if isLoading}
		<div class="grid gap-5 py-4" aria-label={text.loading} aria-busy="true">
			{#each [0, 1, 2, 3] as row (row)}
				<div class="grid gap-2"><Skeleton class="h-5 w-3/4" /><Skeleton class="h-3 w-1/3" /></div>
			{/each}
		</div>
	{:else if visibleFacts.length > 0}
		<div class="grid min-h-96 min-w-0 border-y lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.85fr)]">
			<div class={cn('min-w-0 divide-y lg:max-h-[65svh] lg:overflow-y-auto', selectedFact && 'hidden lg:block')}>
				{#each visibleFacts as fact (memoryFactKey(fact))}
					<button type="button" aria-pressed={selectedKey === memoryFactKey(fact)}
						class={cn('grid w-full gap-3 px-4 py-5 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2', selectedKey === memoryFactKey(fact) && 'bg-muted/60')}
						onclick={() => selectedKey = memoryFactKey(fact)}>
						<p class="line-clamp-3 text-sm leading-6">{fact.content}</p>
						<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
							<span>{memoryAudience(fact, text)}</span>
							<span>{memoryDate(fact.recordedAt ?? fact.validAt, text, currentLocale.value)}</span>
							{#if !isCurrentMemoryFact(fact)}<span>{text.previousMemory}</span>{/if}
						</div>
					</button>
				{/each}
			</div>
			<aside class={cn('min-w-0 lg:border-l', !selectedFact && 'hidden lg:block')} aria-label={text.memoryDetails}>
				{#if selectedFact}
					<div class="px-4 pt-3 lg:hidden"><Button variant="ghost" size="sm" onclick={() => selectedKey = ''}><ArrowLeftIcon data-icon="inline-start" />{text.factListTab}</Button></div>
					{#key memoryFactKey(selectedFact)}
						<MemoryFactDetail fact={selectedFact} episodes={memoryGraph?.episodes ?? []} {text} onChanged={refreshMemory} />
					{/key}
				{:else}
					<Empty.Root class="min-h-96"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Description>{text.selectMemory}</Empty.Description></Empty.Header></Empty.Root>
				{/if}
			</aside>
		</div>
	{:else if memoryGraph && !errorMessage && !isUnavailable && !isIncomplete}
		<Empty.Root class="min-h-80"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Title>{mode === 'search' ? text.noSearchResults : text.noVisibleMemory}</Empty.Title><Empty.Description>{text.searchDescription}</Empty.Description></Empty.Header></Empty.Root>
	{/if}
{/if}
