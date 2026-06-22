<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
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
	const nodes = () =>
		(memoryGraph?.nodes ?? []).map((node) =>
			node.kind === 'namespace' && node.scopeType && isPersonalScope(node.scopeType)
				? { ...node, label: text.myMemory }
				: node
		);
	const edges = () => memoryGraph?.edges ?? [];
	const episodes = () => memoryGraph?.episodes ?? [];
	const hasMemoryGraph = () => Boolean(memoryGraph);
	const hasMemoryHealth = () => Boolean(memoryGraph?.health);

	onMount(loadMemoryGraph);

	function factScoreText(score: number | null | undefined): string {
		if (typeof score !== 'number' || !Number.isFinite(score)) return text.scoreUnavailable;
		return `${text.score} ${Math.round(score * 100)}%`;
	}

	function isPersonalScope(scopeType: string): boolean {
		return scopeType === 'user' || scopeType === 'private';
	}

	function scopeDisplayName(scopeType: string, namespaceID: string): string {
		return isPersonalScope(scopeType) ? text.myMemory : namespaceID;
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

	function healthStatusText(): string | undefined {
		const health = memoryGraph?.health;
		if (!health) return undefined;
		if (health.hasGraphFailure) return text.memoryUnavailable;
		if (health.hasSearchFailure) return text.searchFailed;
		if (health.hasIngestionFailure) return text.ingestionFailed;
		if (health.reachable === true) return text.reachable;
		if (health.reachable === false) return text.unreachable;
		return undefined;
	}

	function healthStatusVariant(): 'secondary' | 'outline' | 'destructive' {
		const health = memoryGraph?.health;
		if (health?.hasGraphFailure || health?.hasSearchFailure || health?.hasIngestionFailure) return 'destructive';
		return health?.reachable ? 'secondary' : 'outline';
	}
</script>

{#if hasMemoryHealth()}
	<div class="flex flex-wrap gap-2">
		<Badge variant={memoryGraph?.health?.configured ? 'secondary' : 'outline'}>
			{memoryGraph?.health?.configured ? text.configured : text.unconfigured}
		</Badge>
		{#if healthStatusText()}
			<Badge variant={healthStatusVariant()}>{healthStatusText()}</Badge>
		{/if}
	</div>
{/if}

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

{#if hasMemoryGraph() && nodes().length === 0}
	<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">{text.noVisibleMemory}</p>
{:else if nodes().length > 0}
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
							<span class="truncate text-xs text-muted-foreground">{scopeDisplayName(fact.scopeType, fact.namespaceID)}</span>
							<span class="ml-auto text-xs tabular-nums text-muted-foreground">{factScoreText(fact.score)}</span>
						</div>
						<div class="memory-markdown text-sm leading-5">
						<SvelteMarkdown source={fact.content} />
					</div>
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
					<p class="truncate text-sm font-medium">{scopeDisplayName(namespace.scopeType, namespace.namespaceID)}</p>
					<p class="text-xs text-muted-foreground">{namespace.scopeType} · {namespace.episodeCount ?? 0} {text.episodes}</p>
				</div>
				{#if !isPersonalScope(namespace.scopeType)}
					<Badge variant="outline">{namespace.scopeCircleID || namespace.scopeConversationID || text.workspace}</Badge>
				{/if}
			</div>
		{/each}
	</section>
{/if}

<style>
	.memory-markdown :global(h1) {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0.25rem 0;
	}
	.memory-markdown :global(h2),
	.memory-markdown :global(h3) {
		font-size: 0.8125rem;
		font-weight: 600;
		margin: 0.5rem 0 0.25rem;
		color: var(--color-muted-foreground);
	}
	.memory-markdown :global(p) {
		margin: 0.25rem 0;
	}
	.memory-markdown :global(ul),
	.memory-markdown :global(ol) {
		margin: 0.25rem 0;
		padding-left: 1.1rem;
	}
	.memory-markdown :global(ul) {
		list-style: disc;
	}
	.memory-markdown :global(ol) {
		list-style: decimal;
	}
	.memory-markdown :global(li) {
		margin: 0.125rem 0;
	}
	.memory-markdown :global(strong) {
		font-weight: 600;
	}
	.memory-markdown :global(em) {
		font-style: italic;
	}
	.memory-markdown :global(a) {
		color: var(--color-primary);
		text-decoration: underline;
	}
	.memory-markdown :global(code) {
		font-family: var(--font-mono, monospace);
		font-size: 0.8125rem;
		background: var(--color-muted);
		padding: 0.05rem 0.25rem;
		border-radius: 0.25rem;
	}
	.memory-markdown :global(pre) {
		margin: 0.375rem 0;
		padding: 0.5rem;
		background: var(--color-muted);
		border-radius: 0.375rem;
		overflow-x: auto;
	}
	.memory-markdown :global(pre code) {
		background: none;
		padding: 0;
	}
	.memory-markdown :global(blockquote) {
		border-left: 2px solid var(--color-border);
		padding-left: 0.5rem;
		color: var(--color-muted-foreground);
		margin: 0.25rem 0;
	}
</style>
