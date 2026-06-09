<script lang="ts">
	import MemoryNetwork from '$lib/components/memory-network.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onMount } from 'svelte';
	import { fetchMemoryGraph, type MemoryGraphResponse } from './memory-graph-api';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	let memoryGraph = $state<MemoryGraphResponse | null>(null);
	let memoryGraphQuery = $state('');
	let errorMessage = $state('');
	let isLoading = $state(false);

	const namespaces = () => memoryGraph?.namespaces ?? [];
	const facts = () => memoryGraph?.facts ?? [];
	const nodes = () => memoryGraph?.nodes ?? [];
	const edges = () => memoryGraph?.edges ?? [];
	const episodes = () => memoryGraph?.episodes ?? [];

	onMount(loadMemoryGraph);

	function factScoreText(score: number | null | undefined): string {
		if (typeof score !== 'number' || !Number.isFinite(score)) return text.scoreUnavailable;
		return `${text.score} ${Math.round(score * 100)}%`;
	}

	async function loadMemoryGraph(): Promise<void> {
		isLoading = true;
		errorMessage = '';
		try {
			memoryGraph = await fetchMemoryGraph(memoryGraphQuery);
		} catch {
			errorMessage = text.loadFailed;
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="flex flex-wrap gap-2">
	<Badge variant={memoryGraph?.health?.configured ? 'secondary' : 'outline'}>
		{memoryGraph?.health?.configured ? text.configured : text.unconfigured}
	</Badge>
	<Badge variant={memoryGraph?.health?.reachable ? 'secondary' : 'outline'}>
		{memoryGraph?.health?.reachable ? text.reachable : text.unreachable}
	</Badge>
</div>

<form
	class="grid gap-2 md:grid-cols-[1fr_auto_auto]"
	onsubmit={(event) => {
		event.preventDefault();
		loadMemoryGraph();
	}}
>
	<Input bind:value={memoryGraphQuery} placeholder={text.searchPlaceholder} autocomplete="off" />
	<Button type="submit" disabled={isLoading} class="gap-2">
		{#if isLoading}
			<LoaderIcon class="size-4 animate-spin" />
		{:else}
			<SearchIcon class="size-4" />
		{/if}
		{text.search}
	</Button>
	<Button type="button" variant="outline" disabled={isLoading} onclick={loadMemoryGraph} class="gap-2">
		<RefreshCwIcon class="size-4" />
		{text.refresh}
	</Button>
</form>

<section class="grid min-w-0 gap-2 sm:grid-cols-3">
	<div class="rounded-md bg-muted/30 p-3">
		<p class="text-2xl font-semibold">{namespaces().length}</p>
		<p class="text-xs text-muted-foreground">{text.namespaces}</p>
	</div>
	<div class="rounded-md bg-muted/30 p-3">
		<p class="text-2xl font-semibold">{episodes().length}</p>
		<p class="text-xs text-muted-foreground">{text.episodes}</p>
	</div>
	<div class="rounded-md bg-muted/30 p-3">
		<p class="text-2xl font-semibold">{facts().length}</p>
		<p class="text-xs text-muted-foreground">{text.facts}</p>
	</div>
</section>

{#if errorMessage}
	<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
{/if}

{#if memoryGraph?.health?.error || memoryGraph?.health?.lastSearchError || memoryGraph?.health?.lastIngestionError}
	<section class="grid gap-2 text-xs">
		{#if memoryGraph.health.error}
			<p class="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-amber-950">{memoryGraph.health.error}</p>
		{/if}
		{#if memoryGraph.health.lastSearchError}
			<p class="rounded-md border bg-muted/30 px-3 py-2">{text.searchError}: {memoryGraph.health.lastSearchError}</p>
		{/if}
		{#if memoryGraph.health.lastIngestionError}
			<p class="rounded-md border bg-muted/30 px-3 py-2">{text.ingestionError}: {memoryGraph.health.lastIngestionError}</p>
		{/if}
	</section>
{/if}

{#if nodes().length === 0}
	<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">{text.noVisibleMemory}</p>
{:else}
	<section class="grid min-w-0 gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
		<MemoryNetwork nodes={nodes()} edges={edges()} />
		<aside class="flex h-[min(58svh,520px)] min-h-[360px] min-w-0 flex-col overflow-hidden rounded-lg border bg-background">
			<div class="flex items-center justify-between gap-3 border-b px-3 py-2">
				<div class="min-w-0">
					<h2 class="truncate text-sm font-semibold">{text.memoryDetails}</h2>
					<p class="text-xs text-muted-foreground">{facts().length} {text.facts}</p>
				</div>
			</div>
			<div class="min-h-0 overflow-y-auto">
				{#each facts() as fact}
					<article class="grid gap-2 border-b px-3 py-3 last:border-b-0">
						<div class="flex min-w-0 flex-wrap items-center gap-2">
							<Badge variant="outline">{fact.sourceKind ?? text.source}</Badge>
							<span class="truncate text-xs text-muted-foreground">{fact.namespaceID}</span>
							<span class="ml-auto text-xs tabular-nums text-muted-foreground">{factScoreText(fact.score)}</span>
						</div>
						<p class="text-sm leading-5">{fact.content}</p>
					</article>
				{/each}
			</div>
		</aside>
	</section>
{/if}

{#if namespaces().length > 0}
	<section class="overflow-hidden rounded-lg border">
		{#each namespaces().slice(0, 10) as namespace}
			<div class="flex flex-wrap items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0">
				<div class="min-w-0">
					<p class="truncate text-sm font-medium">{namespace.namespaceID}</p>
					<p class="text-xs text-muted-foreground">{namespace.scopeType} · {namespace.episodeCount ?? 0} {text.episodes}</p>
				</div>
				<Badge variant="outline">{namespace.scopeCircleID || namespace.scopePersonID || namespace.scopeConversationID || text.workspace}</Badge>
			</div>
		{/each}
	</section>
{/if}
