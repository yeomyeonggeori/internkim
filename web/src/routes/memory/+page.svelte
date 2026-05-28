<script lang="ts">
	import MemoryNetwork from '$lib/components/memory-network.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onMount } from 'svelte';

	type MemoryGraphHealth = {
		configured?: boolean;
		reachable?: boolean;
		lastSearchError?: string;
		lastIngestionError?: string;
		error?: string;
	};

	type MemoryGraphNamespace = {
		namespaceID: string;
		scopeType: string;
		scopePersonID?: string;
		scopeConversationID?: string;
		scopeCircleID?: string;
		episodeCount?: number;
	};

	type MemoryGraphFact = {
		factID: string;
		scopeType: string;
		namespaceID: string;
		content: string;
		score?: number;
		sourceKind?: string;
	};

	type MemoryGraphNode = {
		nodeID: string;
		label: string;
		kind: string;
		scopeType?: string;
		status?: string;
	};

	type MemoryGraphEdge = {
		sourceID: string;
		targetID: string;
		weight?: number;
	};

	type MemoryGraphResponse = {
		health?: MemoryGraphHealth;
		namespaces?: MemoryGraphNamespace[];
		episodes?: unknown[];
		facts?: MemoryGraphFact[];
		nodes?: MemoryGraphNode[];
		edges?: MemoryGraphEdge[];
	};

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

	async function loadMemoryGraph() {
		isLoading = true;
		errorMessage = '';
		try {
			const urlParams = new URLSearchParams({ limit: '120' });
			if (memoryGraphQuery.trim()) urlParams.set('query', memoryGraphQuery.trim());
			const response = await fetch(`/memory/api/graph?${urlParams.toString()}`, { credentials: 'include' });
			if (!response.ok) {
				errorMessage = `Memory graph could not be loaded (${response.status}).`;
				return;
			}
			memoryGraph = (await response.json()) as MemoryGraphResponse;
		} catch {
			errorMessage = 'Memory graph could not be loaded.';
		} finally {
			isLoading = false;
		}
	}
</script>

<svelte:head>
	<title>Memory · intern kim</title>
</svelte:head>

<main class="grid min-h-[calc(100svh-48px)] gap-5 overflow-x-hidden px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<section class="flex min-w-0 flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="flex items-center gap-2 text-xl font-semibold">
				<NetworkIcon class="size-5 text-teal-700" />
				Memory
			</h1>
			<p class="mt-1 max-w-full text-sm text-muted-foreground">Your visible Blueclaw memory as a network graph.</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Badge variant={memoryGraph?.health?.configured ? 'secondary' : 'outline'}>
				{memoryGraph?.health?.configured ? 'configured' : 'unconfigured'}
			</Badge>
			<Badge variant={memoryGraph?.health?.reachable ? 'secondary' : 'outline'}>
				{memoryGraph?.health?.reachable ? 'reachable' : 'unreachable'}
			</Badge>
		</div>
	</section>

	<form
		class="grid gap-2 md:grid-cols-[1fr_auto_auto]"
		onsubmit={(event) => {
			event.preventDefault();
			loadMemoryGraph();
		}}
	>
		<Input bind:value={memoryGraphQuery} placeholder="Search memory facts" autocomplete="off" />
		<Button type="submit" disabled={isLoading} class="gap-2">
			{#if isLoading}
				<LoaderIcon class="size-4 animate-spin" />
			{:else}
				<SearchIcon class="size-4" />
			{/if}
			Search
		</Button>
		<Button type="button" variant="outline" disabled={isLoading} onclick={loadMemoryGraph} class="gap-2">
			<RefreshCwIcon class="size-4" />
			Refresh
		</Button>
	</form>

	<section class="grid min-w-0 gap-2 sm:grid-cols-3">
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-2xl font-semibold">{namespaces().length}</p>
			<p class="text-xs text-muted-foreground">namespaces</p>
		</div>
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-2xl font-semibold">{episodes().length}</p>
			<p class="text-xs text-muted-foreground">episodes</p>
		</div>
		<div class="rounded-md bg-muted/30 p-3">
			<p class="text-2xl font-semibold">{facts().length}</p>
			<p class="text-xs text-muted-foreground">facts</p>
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
				<p class="rounded-md border bg-muted/30 px-3 py-2">search: {memoryGraph.health.lastSearchError}</p>
			{/if}
			{#if memoryGraph.health.lastIngestionError}
				<p class="rounded-md border bg-muted/30 px-3 py-2">ingestion: {memoryGraph.health.lastIngestionError}</p>
			{/if}
		</section>
	{/if}

	{#if nodes().length === 0}
		<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">No visible memory yet.</p>
	{:else}
		<MemoryNetwork nodes={nodes()} edges={edges()} />
	{/if}

	{#if namespaces().length > 0}
		<section class="overflow-hidden rounded-lg border">
			{#each namespaces().slice(0, 10) as namespace}
				<div class="flex flex-wrap items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0">
					<div class="min-w-0">
						<p class="truncate text-sm font-medium">{namespace.namespaceID}</p>
						<p class="text-xs text-muted-foreground">{namespace.scopeType} · {namespace.episodeCount ?? 0} episodes</p>
					</div>
					<Badge variant="outline">{namespace.scopeCircleID || namespace.scopePersonID || namespace.scopeConversationID || 'workspace'}</Badge>
				</div>
			{/each}
		</section>
	{/if}
</main>
