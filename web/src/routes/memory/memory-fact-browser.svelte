<script lang="ts">
	import { onMount } from 'svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import SearchIcon from '@lucide/svelte/icons/search';
	import RefreshIcon from '@lucide/svelte/icons/refresh-cw';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
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
	import { fetchMemoryFacts, type MemoryFactsResponse } from './memory-facts-api';
	import { filterMemoryFacts, isLiveMemoryFact, memoryAudience, memoryDate, memoryKindLabel } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();
	let memory = $state<MemoryFactsResponse | null>(null);
	let selectedFactID = $state('');
	let query = $state('');
	let isLoading = $state(false);
	let includesPrevious = $state(false);
	let hasLoadError = $state(false);
	let requestSequence = 0;
	const errorMessage = $derived(hasLoadError ? text.loadFailed : '');
	const identityLines = $derived(memory?.profile.identityLines ?? []);
	const searchedFacts = $derived(filterMemoryFacts(memory?.facts ?? [], query));
	const visibleFacts = $derived(searchedFacts.filter((fact) => includesPrevious || isLiveMemoryFact(fact)));
	const selectedFact = $derived(visibleFacts.find((fact) => fact.factID === selectedFactID));
	const isSearching = $derived(query.trim().length > 0);

	onMount(() => { void loadMemory(); });

	async function loadMemory(): Promise<void> {
		const requestID = ++requestSequence;
		isLoading = true;
		hasLoadError = false;
		memory = null;
		try {
			const response = await fetchMemoryFacts();
			if (requestID !== requestSequence) return;
			memory = response;
		} catch {
			if (requestID === requestSequence) hasLoadError = true;
		} finally {
			if (requestID === requestSequence) isLoading = false;
		}
	}

	function clearSearch(): void {
		query = '';
		selectedFactID = '';
	}

	function removeForgottenFact(factID: string): void {
		if (!memory) return;
		memory = { ...memory, facts: memory.facts.filter((fact) => fact.factID !== factID) };
		if (selectedFactID === factID) selectedFactID = '';
	}
</script>

<div class="flex min-w-0 flex-col rounded-xl border bg-background">
	<div class="flex min-w-0 flex-col gap-4 p-4">
	<form role="search" class="flex flex-col gap-3" onsubmit={(event) => event.preventDefault()}>
		<Field.FieldGroup>
			<Field.Field>
				<Field.Label for="memory-search" class="sr-only">{text.searchPrompt}</Field.Label>
				<InputGroup.Root>
					<InputGroup.Addon align="inline-start"><SearchIcon /></InputGroup.Addon>
					<InputGroup.Input id="memory-search" bind:value={query} placeholder={text.searchPlaceholder} />
					{#if query}
						<InputGroup.Addon align="inline-end">
							<InputGroup.Button type="button" size="icon-xs" onclick={clearSearch} aria-label={text.clearSearch}><XIcon /></InputGroup.Button>
						</InputGroup.Addon>
					{/if}
				</InputGroup.Root>
			</Field.Field>
		</Field.FieldGroup>
	</form>

	<div class="flex flex-wrap items-center justify-between gap-3">
		<p class="min-w-0 text-sm text-muted-foreground" aria-live="polite">{isLoading ? text.loading : errorMessage ? '' : `${isSearching ? text.searchResults : text.currentMemories} · ${text.visibleCountTemplate.replace('{count}', String(visibleFacts.length))}`}</p>
		<div class="flex shrink-0 items-center gap-3">
			<Field.Field orientation="horizontal" class="w-auto">
				<Checkbox id="memory-include-previous" bind:checked={includesPrevious} />
				<Field.Content><Field.Label for="memory-include-previous">{text.showPrevious}</Field.Label></Field.Content>
			</Field.Field>
			<Button variant="ghost" size="icon-sm" disabled={isLoading} onclick={loadMemory} aria-label={text.refresh}><RefreshIcon /></Button>
		</div>
	</div>
	</div>
	{#if errorMessage}
		<div class="px-4 pb-4">
			<Alert.Root variant="destructive">
				<Alert.Description>{errorMessage}</Alert.Description>
				<Alert.Action><Button variant="outline" size="sm" disabled={isLoading} onclick={loadMemory}>{text.retry}</Button></Alert.Action>
			</Alert.Root>
		</div>
	{/if}
	{#if isLoading}
		<div class="grid min-h-96 content-start gap-5 border-t p-5" aria-label={text.loading} aria-busy="true">
			{#each [0, 1, 2, 3] as row (row)}
				<div class="grid gap-2"><Skeleton class="h-5 w-3/4" /><Skeleton class="h-3 w-1/3" /></div>
			{/each}
		</div>
	{:else if memory}
		<section class="grid gap-2 border-t px-4 py-4" aria-label={text.profile}>
			<h2 class="flex items-center gap-2 text-sm font-semibold"><UserRoundIcon class="size-4 text-muted-foreground" />{text.profile}</h2>
			<p class="text-xs text-muted-foreground">{text.profileDescription}</p>
			{#if identityLines.length > 0}
				<ul class="grid gap-1 text-sm leading-6">
					{#each identityLines as line, index (index)}<li class="break-words">{line}</li>{/each}
				</ul>
			{:else}
				<p class="text-sm text-muted-foreground">{text.profileEmpty}</p>
			{/if}
		</section>
		{#if visibleFacts.length > 0}
			<div class="grid min-w-0 border-t lg:min-h-[28rem] lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1fr)]">
				<div class={cn('min-w-0 divide-y lg:max-h-[65svh] lg:overflow-y-auto', selectedFact && 'hidden lg:block')}>
					{#each visibleFacts as fact (fact.factID)}
						<button type="button" aria-pressed={selectedFactID === fact.factID}
							class={cn('grid w-full gap-3 px-4 py-5 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2', selectedFactID === fact.factID && 'bg-muted/60')}
							onclick={() => selectedFactID = fact.factID}>
							<p class="line-clamp-3 break-words text-sm leading-6">{fact.content}</p>
							<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
								<span>{memoryKindLabel(fact.kind, text)}</span>
								<span>{memoryAudience(fact, text)}</span>
								<span>{memoryDate(fact.validFrom, text, currentLocale.value)}</span>
								{#if !isLiveMemoryFact(fact)}<Badge variant="secondary">{text.previousMemory}</Badge>{/if}
							</div>
						</button>
					{/each}
				</div>
				<aside class={cn('min-w-0 lg:max-h-[65svh] lg:overflow-y-auto lg:border-l', !selectedFact && 'hidden lg:block')} aria-label={text.memoryDetails}>
					{#if selectedFact}
						<div class="px-4 pt-3 lg:hidden"><Button variant="ghost" size="sm" onclick={() => selectedFactID = ''}><ArrowLeftIcon data-icon="inline-start" />{text.factListTab}</Button></div>
						{#key selectedFact.factID}
							<MemoryFactDetail fact={selectedFact} {text} onForgotten={removeForgottenFact} />
						{/key}
					{:else}
						<Empty.Root class="min-h-[28rem]"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Title>{text.memoryDetails}</Empty.Title><Empty.Description>{text.selectMemory}</Empty.Description></Empty.Header></Empty.Root>
					{/if}
				</aside>
			</div>
		{:else}
			<Empty.Root class="min-h-80 border-t"><Empty.Header><Empty.Media variant="icon"><BookOpenIcon /></Empty.Media><Empty.Title>{isSearching ? text.noSearchResults : text.noVisibleMemory}</Empty.Title><Empty.Description>{isSearching ? text.noSearchDescription : text.browseDescription}</Empty.Description></Empty.Header></Empty.Root>
		{/if}
	{/if}
</div>
