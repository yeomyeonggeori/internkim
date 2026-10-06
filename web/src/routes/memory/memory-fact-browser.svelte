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
	import MemoryImportance from './memory-importance.svelte';
	import MemoryLayerStack from './memory-layer-stack.svelte';
	import type { Circle } from '$lib/data-room/model';
	import { fetchCircles, fetchMemoryFacts, type MemoryLayer, type MemoryFactsResponse } from './memory-facts-api';
	import { filterMemoryFacts, groupFactsByLayer, isCurrentMemory, memoryLayerKey, memoryScopeLabel, memoryWhen } from './memory-workbench-model';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();
	let memory = $state<MemoryFactsResponse | null>(null);
	let circles = $state<Circle[]>([]);
	let selectedFactID = $state('');
	let selectedLayerKey = $state('all');
	let query = $state('');
	let isLoading = $state(false);
	let includesPrevious = $state(false);
	let hasLoadError = $state(false);
	let requestSequence = 0;
	const errorMessage = $derived(hasLoadError ? text.loadFailed : '');
	const scopeLabel = (layer: MemoryLayer) => memoryScopeLabel(layer, circles, text, currentLocale.value);
	const countOf = (key: string) => (memory?.facts ?? []).filter((fact) => memoryLayerKey(fact) === key && isCurrentMemory(fact)).length;
	const searchedFacts = $derived(filterMemoryFacts(memory?.facts ?? [], query, scopeLabel));
	const visibleFacts = $derived(searchedFacts.filter((fact) => (includesPrevious || isCurrentMemory(fact)) && (selectedLayerKey === 'all' || memoryLayerKey(fact) === selectedLayerKey)));
	const factGroups = $derived(groupFactsByLayer(memory?.layers ?? [], visibleFacts));
	const selectedFact = $derived(visibleFacts.find((fact) => fact.factID === selectedFactID));
	const isSearching = $derived(query.trim().length > 0);

	onMount(() => { void loadMemory(); });

	async function loadMemory(): Promise<void> {
		const requestID = ++requestSequence;
		isLoading = true;
		hasLoadError = false;
		memory = null;
		try {
			const [response, knownCircles] = await Promise.all([fetchMemoryFacts(), fetchCircles()]);
			if (requestID !== requestSequence) return;
			memory = response;
			circles = knownCircles;
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

<div class="grid min-w-0 gap-6 lg:grid-cols-[16rem_minmax(0,1fr)] lg:items-start">
<aside class="flex min-w-0 flex-col gap-5" aria-label={text.layersTitle}>
	{#if memory}
		<MemoryLayerStack layers={memory.layers} selectedKey={selectedLayerKey} {countOf} labelOf={scopeLabel} onSelect={(key) => { selectedLayerKey = key; selectedFactID = ''; }} {text} />
	{/if}
</aside>
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
		{#if visibleFacts.length > 0}
			<div class="grid min-w-0 border-t xl:min-h-[28rem] xl:grid-cols-[minmax(0,0.85fr)_minmax(0,1fr)]">
				<div class={cn('min-w-0 xl:max-h-[65svh] xl:overflow-y-auto', selectedFact && 'hidden xl:block')}>
					{#each factGroups as group (memoryLayerKey(group.layer))}
						<section aria-label={scopeLabel(group.layer)}>
							<h3 class="flex items-center gap-2 border-b bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground">
								<span class="text-foreground">{scopeLabel(group.layer)}</span>
							</h3>
							<div class="divide-y border-b">
								{#each group.facts as fact (fact.factID)}
									<button type="button" aria-pressed={selectedFactID === fact.factID}
										class={cn('grid w-full grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-3 px-4 py-5 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2', selectedFactID === fact.factID && 'bg-muted/60')}
										onclick={() => selectedFactID = fact.factID}>
										<p class="line-clamp-3 break-words text-sm leading-6">{fact.content}</p>
										<MemoryImportance class="mt-1.5" importance={fact.importance} label={`${text.importance} ${text.importanceTemplate.replace('{count}', String(fact.importance))}`} />
										<div class="col-span-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
											<span>{memoryWhen(fact, text, currentLocale.value)}</span>
											{#if !isCurrentMemory(fact)}<Badge variant="secondary">{text.previousMemory}</Badge>{/if}
										</div>
									</button>
								{/each}
							</div>
						</section>
					{/each}
				</div>
				<aside class={cn('min-w-0 xl:max-h-[65svh] xl:overflow-y-auto xl:border-l', !selectedFact && 'hidden xl:block')} aria-label={text.memoryDetails}>
					{#if selectedFact}
						<div class="px-4 pt-3 xl:hidden"><Button variant="ghost" size="sm" onclick={() => selectedFactID = ''}><ArrowLeftIcon data-icon="inline-start" />{text.factListTab}</Button></div>
						{#key selectedFact.factID}
							<MemoryFactDetail fact={selectedFact} scope={scopeLabel(selectedFact)} {text} onForgotten={removeForgottenFact} />
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
</div>
