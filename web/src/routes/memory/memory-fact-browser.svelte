<script lang="ts">
	import { onMount } from 'svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import SearchIcon from '@lucide/svelte/icons/search';
	import RefreshIcon from '@lucide/svelte/icons/refresh-cw';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import XIcon from '@lucide/svelte/icons/x';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import * as InputGroup from '$lib/components/ui/input-group';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Empty from '$lib/components/ui/empty';
	import { cn } from '$lib/utils';
	import MemoryFactDetail from './memory-fact-detail.svelte';
	import { fetchMemoryGraph, type MemoryGraphResponse } from './memory-graph-api';
	import { memoryFactKey, isCurrentMemoryFact, memoryAudience, memoryDate } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();
	let memoryGraph = $state<MemoryGraphResponse | null>(null);
	let selectedKey = $state('');
	let query = $state('');
	let submittedQuery = $state('');
	let isLoading = $state(false);
	let includesPrevious = $state(false);
	let hasLoadError = $state(false);
	const errorMessage = $derived(hasLoadError ? text.loadFailed : '');
	let requestSequence = 0;
	const rememberedFacts = $derived((memoryGraph?.facts ?? []).filter((fact) => fact.sourceKind === 'fact'));
	const visibleFacts = $derived(rememberedFacts.filter((fact) => includesPrevious || isCurrentMemoryFact(fact)));
	const selectedFact = $derived(visibleFacts.find((fact) => memoryFactKey(fact) === selectedKey));
	const isUnavailable = $derived(memoryGraph?.health?.configured === false || memoryGraph?.health?.reachable === false);
	const isIncomplete = $derived(memoryGraph?.retrieval?.complete === false);

	onMount(() => { void loadMemory(''); });

	async function loadMemory(searchQuery: string): Promise<void> {
		const requestID = ++requestSequence;
		isLoading = true;
		hasLoadError = false;
		submittedQuery = searchQuery;
		memoryGraph = null;
		try {
			const response = await fetchMemoryGraph(searchQuery);
			if (requestID !== requestSequence) return;
			memoryGraph = response;
		} catch {
			if (requestID === requestSequence) hasLoadError = true;
		} finally {
			if (requestID === requestSequence) isLoading = false;
		}
	}

	function searchMemory(event: SubmitEvent): void {
		event.preventDefault();
		if (isLoading) return;
		selectedKey = '';
		void loadMemory(query.trim());
	}

	function clearSearch(): void {
		query = '';
		submittedQuery = '';
		selectedKey = '';
		void loadMemory('');
	}

	async function refreshMemory(): Promise<void> {
		await loadMemory(submittedQuery);
	}
</script>

<div class="flex min-w-0 flex-col rounded-xl border bg-background">
	<div class="flex min-w-0 flex-col gap-4 p-4">
	<form role="search" class="flex flex-col gap-3" onsubmit={searchMemory}>
		<Field.FieldGroup>
			<Field.Field>
				<Field.Label for="memory-search" class="sr-only">{text.searchPrompt}</Field.Label>
				<InputGroup.Root>
					<InputGroup.Input id="memory-search" bind:value={query} placeholder={text.searchPlaceholder} />
					<InputGroup.Addon align="inline-end">
						{#if query || submittedQuery}
							<InputGroup.Button type="button" size="icon-xs" onclick={clearSearch} aria-label={text.clearSearch}><XIcon /></InputGroup.Button>
						{/if}
						<InputGroup.Button type="submit" size="xs" disabled={isLoading}><SearchIcon data-icon="inline-start" />{text.search}</InputGroup.Button>
					</InputGroup.Addon>
				</InputGroup.Root>
			</Field.Field>
		</Field.FieldGroup>
	</form>

	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="min-w-0 text-sm text-muted-foreground" aria-live="polite">{isLoading ? text.loading : errorMessage || isUnavailable ? '' : `${submittedQuery ? text.searchResults : text.currentMemories} · ${text.visibleCountTemplate.replace('{count}', String(visibleFacts.length))}`}</p>
		<div class="flex shrink-0 items-center gap-3">
			<Field.Field orientation="horizontal" class="w-auto">
				<Checkbox id="memory-include-previous" bind:checked={includesPrevious} />
				<Field.Content><Field.Label for="memory-include-previous">{text.showPrevious}</Field.Label></Field.Content>
			</Field.Field>
			<Button variant="ghost" size="icon-sm" disabled={isLoading} onclick={refreshMemory} aria-label={text.refresh}><RefreshIcon /></Button>
		</div>
	</div>
	</div>
	{#if errorMessage || isUnavailable || isIncomplete}
		<div class="px-4 pb-4">
			<Alert.Root variant={errorMessage || isUnavailable ? 'destructive' : 'default'}>
				<Alert.Description>{errorMessage || (isUnavailable ? text.unreachable : text.incompleteResults)}</Alert.Description>
				<Alert.Action><Button variant="outline" size="sm" disabled={isLoading} onclick={refreshMemory}>{text.retry}</Button></Alert.Action>
			</Alert.Root>
		</div>
	{/if}
	{#if isLoading}
		<div class="grid min-h-96 content-start gap-5 border-t p-5" aria-label={text.loading} aria-busy="true">
			{#each [0, 1, 2, 3] as row (row)}
				<div class="grid gap-2"><Skeleton class="h-5 w-3/4" /><Skeleton class="h-3 w-1/3" /></div>
			{/each}
		</div>
	{:else if visibleFacts.length > 0}
		<div class="grid min-w-0 border-t lg:min-h-[28rem] lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1fr)]">
			<div class={cn('min-w-0 divide-y lg:max-h-[65svh] lg:overflow-y-auto', selectedFact && 'hidden lg:block')}>
				{#each visibleFacts as fact (memoryFactKey(fact))}
					<button type="button" aria-pressed={selectedKey === memoryFactKey(fact)}
						class={cn('grid w-full gap-3 px-4 py-5 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2', selectedKey === memoryFactKey(fact) && 'bg-muted/60')}
						onclick={() => selectedKey = memoryFactKey(fact)}>
						<p class="line-clamp-3 break-words text-sm leading-6">{fact.content}</p>
						<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
							<span>{memoryAudience(fact, text)}</span>
							<span>{memoryDate(fact.recordedAt ?? fact.validAt, text, currentLocale.value)}</span>
							{#if !isCurrentMemoryFact(fact)}<Badge variant="secondary">{text.previousMemory}</Badge>{/if}
						</div>
					</button>
				{/each}
			</div>
			<aside class={cn('min-w-0 lg:max-h-[65svh] lg:overflow-y-auto lg:border-l', !selectedFact && 'hidden lg:block')} aria-label={text.memoryDetails}>
				{#if selectedFact}
					<div class="px-4 pt-3 lg:hidden"><Button variant="ghost" size="sm" onclick={() => selectedKey = ''}><ArrowLeftIcon data-icon="inline-start" />{text.factListTab}</Button></div>
					{#key memoryFactKey(selectedFact)}
						<MemoryFactDetail fact={selectedFact} episodes={memoryGraph?.episodes ?? []} {text} onChanged={refreshMemory} />
					{/key}
				{:else}
					<Empty.Root class="min-h-[28rem]"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Title>{text.memoryDetails}</Empty.Title><Empty.Description>{text.selectMemory}</Empty.Description></Empty.Header></Empty.Root>
				{/if}
			</aside>
		</div>
	{:else if memoryGraph && !errorMessage && !isUnavailable && !isIncomplete}
		<Empty.Root class="min-h-80"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Title>{submittedQuery ? text.noSearchResults : text.noVisibleMemory}</Empty.Title><Empty.Description>{submittedQuery ? text.noSearchDescription : text.browseDescription}</Empty.Description></Empty.Header></Empty.Root>
	{/if}
</div>
