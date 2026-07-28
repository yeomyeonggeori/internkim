<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import MemoryNetwork from '$lib/components/memory-network.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SaveIcon from '@lucide/svelte/icons/save';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import { onMount } from 'svelte';
	import {
		deleteMemoryEpisode,
		deletePinnedMemory,
		fetchMemoryGraph,
		savePinnedMemory,
		type MemoryGraphResponse,
		type MemoryGraphNode,
		type MemoryGraphEdge,
		type MemoryGraphEpisode
	} from './memory-graph-api';
	import { factsForMemoryGraphSelection, isPersonalScope, mergedPersonalNodeID } from './memory-graph-selection';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	let memoryGraph = $state<MemoryGraphResponse | null>(null);
	let memoryGraphQuery = $state('');
	let errorMessage = $state('');
	let actionErrorMessage = $state('');
	let isLoading = $state(false);
	let isSavingMemory = $state(false);
	let isEditingPinnedMemory = $state(false);
	let pinnedMemoryDraft = $state('');
	let selectedNode = $state<MemoryGraphNode | null>(null);
	let showAllNamespaces = $state(false);

	const namespaces = () => memoryGraph?.namespaces ?? [];
	const facts = () => memoryGraph?.facts ?? [];
	const graphTopology = $derived(mergePersonalNamespaceNodes(memoryGraph?.nodes ?? [], memoryGraph?.edges ?? []));
	const nodes = () => graphTopology.nodes;
	const edges = () => graphTopology.edges;
	const selectedEpisode = (): MemoryGraphEpisode | null => {
		if (selectedNode?.kind !== 'episode') return null;
		return (memoryGraph?.episodes ?? []).find((episode) => `episode:${episode.episodeID}` === selectedNode?.nodeID) ?? null;
	};
	const visibleFacts = () => factsForMemoryGraphSelection(facts(), selectedNode, selectedEpisode());
	const pinnedMemoryFact = () => visibleFacts().find((fact) => fact.sourceKind === 'pinned');
	const canEditPinnedMemory = () =>
		selectedNode?.kind === 'namespace' &&
		(selectedNode.nodeID === mergedPersonalNodeID || isPersonalScope(selectedNode.scopeType ?? ''));

	function selectNode(nodeID: string): void {
		selectedNode = nodes().find((node) => node.nodeID === nodeID) ?? null;
		actionErrorMessage = '';
		isEditingPinnedMemory = false;
	}

	function clearSelection(): void {
		selectedNode = null;
		actionErrorMessage = '';
		isEditingPinnedMemory = false;
	}

	const episodes = () => memoryGraph?.episodes ?? [];
	const hasMemoryGraph = () => Boolean(memoryGraph);
	const hasMemoryHealth = () => Boolean(memoryGraph?.health);

	onMount(loadMemoryGraph);

	function factScoreText(score: number | null | undefined): string {
		if (typeof score !== 'number' || !Number.isFinite(score)) return text.scoreUnavailable;
		return `${text.score} ${Math.round(score * 100)}%`;
	}

	function scopeDisplayName(scopeType: string, namespaceID: string): string {
		return isPersonalScope(scopeType) ? text.myMemory : namespaceID;
	}

	const visibleNamespaces = () => (showAllNamespaces ? namespaces() : namespaces().slice(0, 10));
	const namespacesShowAllText = () => text.namespacesShowAllTemplate.replace('{count}', String(namespaces().length));

	function mergePersonalNamespaceNodes(
		rawNodes: MemoryGraphNode[],
		rawEdges: MemoryGraphEdge[]
	): { nodes: MemoryGraphNode[]; edges: MemoryGraphEdge[] } {
		const personalNodeIDs = new Set(
			rawNodes
				.filter((node) => node.kind === 'namespace' && node.scopeType && isPersonalScope(node.scopeType))
				.map((node) => node.nodeID)
		);
		if (personalNodeIDs.size === 0) return { nodes: rawNodes, edges: rawEdges };

		const remapNodeID = (nodeID: string) => (personalNodeIDs.has(nodeID) ? mergedPersonalNodeID : nodeID);

		const nodes: MemoryGraphNode[] = [];
		let hasMergedNode = false;
		for (const node of rawNodes) {
			if (!personalNodeIDs.has(node.nodeID)) {
				nodes.push(node);
				continue;
			}
			if (hasMergedNode) continue;
			nodes.push({ ...node, nodeID: mergedPersonalNodeID, label: text.myMemory });
			hasMergedNode = true;
		}

		const seenEdgeKeys = new Set<string>();
		const edges: MemoryGraphEdge[] = [];
		for (const edge of rawEdges) {
			const sourceID = remapNodeID(edge.sourceID);
			const targetID = remapNodeID(edge.targetID);
			if (sourceID === targetID) continue;
			const edgeKey = `${sourceID}->${targetID}`;
			if (seenEdgeKeys.has(edgeKey)) continue;
			seenEdgeKeys.add(edgeKey);
			edges.push({ ...edge, sourceID, targetID });
		}
		return { nodes, edges };
	}

	async function loadMemoryGraph(): Promise<void> {
		isLoading = true;
		errorMessage = '';
		actionErrorMessage = '';
		selectedNode = null;
		isEditingPinnedMemory = false;
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

	function startPinnedMemoryEdit(): void {
		pinnedMemoryDraft = pinnedMemoryFact()?.content ?? '# Memory\n';
		actionErrorMessage = '';
		isEditingPinnedMemory = true;
	}

	async function savePinnedMemoryDraft(): Promise<void> {
		isSavingMemory = true;
		actionErrorMessage = '';
		try {
			await savePinnedMemory(pinnedMemoryDraft);
			isEditingPinnedMemory = false;
			await loadMemoryGraph();
		} catch {
			actionErrorMessage = text.memorySaveFailed;
		} finally {
			isSavingMemory = false;
		}
	}

	function confirmDeletePinnedMemory(): void {
		confirmDelete({
			title: text.memoryDeleteTitle,
			description: text.pinnedMemoryDeleteDescription,
			confirm: { text: text.memoryDeleteAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await deleteSelectedPinnedMemory();
			}
		});
	}

	async function deleteSelectedPinnedMemory(): Promise<void> {
		actionErrorMessage = '';
		try {
			await deletePinnedMemory();
			await loadMemoryGraph();
		} catch {
			actionErrorMessage = text.memoryDeleteFailed;
		}
	}

	function confirmDeleteEpisode(episode: MemoryGraphEpisode): void {
		confirmDelete({
			title: text.memoryDeleteTitle,
			description: text.episodeDeleteDescription,
			confirm: { text: text.memoryDeleteAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await deleteSelectedEpisode(episode);
			}
		});
	}

	async function deleteSelectedEpisode(episode: MemoryGraphEpisode): Promise<void> {
		actionErrorMessage = '';
		try {
			await deleteMemoryEpisode(episode.episodeID, episode.namespaceIDs ?? []);
			await loadMemoryGraph();
		} catch {
			actionErrorMessage = text.memoryDeleteFailed;
		}
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
	<Button type="button" variant="ghost" size="icon-sm" disabled={isLoading} onclick={loadMemoryGraph} aria-label={text.refresh} title={text.refresh}>
		<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
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
		<MemoryNetwork
				nodes={nodes()}
				edges={edges()}
				selectedNodeID={selectedNode?.nodeID ?? null}
				onNodeSelect={selectNode}
				onClearSelection={clearSelection}
			/>
		<aside class="flex h-[min(58svh,520px)] min-h-[360px] min-w-0 flex-col overflow-hidden rounded-lg border bg-background">
			<div class="flex items-center justify-between gap-3 border-b px-3 py-2">
				<div class="min-w-0">
					<h2 class="truncate text-sm font-semibold">{selectedNode ? selectedNode.label : text.memoryDetails}</h2>
					<p class="text-xs text-muted-foreground">{visibleFacts().length} {text.facts}</p>
				</div>
				{#if selectedNode}
					<Button type="button" variant="ghost" size="sm" onclick={clearSelection}>{text.viewAll}</Button>
				{/if}
			</div>
			<div class="min-h-0 overflow-y-auto">
				{#if actionErrorMessage}
					<p class="m-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{actionErrorMessage}</p>
				{/if}
				{#if canEditPinnedMemory()}
					<article class="grid gap-2 border-b px-3 py-3">
						<div class="flex min-w-0 flex-wrap items-center gap-2">
							<Badge variant="outline">MEMORY.md</Badge>
							<div class="ml-auto flex items-center gap-1">
								<Button type="button" variant="ghost" size="icon-sm" onclick={startPinnedMemoryEdit} aria-label={text.memoryEdit} title={text.memoryEdit}>
									<PencilIcon class="size-4" />
								</Button>
								<Button type="button" variant="ghost" size="icon-sm" onclick={confirmDeletePinnedMemory} aria-label={text.memoryDelete} title={text.memoryDelete} disabled={!pinnedMemoryFact()}>
									<TrashIcon class="size-4" />
								</Button>
							</div>
						</div>
						{#if isEditingPinnedMemory}
							<Textarea bind:value={pinnedMemoryDraft} class="min-h-52 resize-y font-mono text-sm" />
							<div class="flex justify-end gap-2">
								<Button type="button" variant="outline" size="sm" onclick={() => (isEditingPinnedMemory = false)} disabled={isSavingMemory}>{text.cancel}</Button>
								<Button type="button" size="sm" onclick={() => void savePinnedMemoryDraft()} disabled={isSavingMemory} class="gap-2">
									{#if isSavingMemory}
										<LoaderIcon class="size-4 animate-spin" />
									{:else}
										<SaveIcon class="size-4" />
									{/if}
									{text.save}
								</Button>
							</div>
						{/if}
					</article>
				{/if}
				{#if selectedEpisode()}
					{@const episode = selectedEpisode()}
					<article class="grid gap-2 border-b px-3 py-3">
						<div class="flex min-w-0 flex-wrap items-center gap-2">
							<Badge variant={episode?.ingestionStatus === 'failed' ? 'destructive' : 'outline'}>
								{episode?.ingestionStatus ?? text.episodes}
							</Badge>
							<span class="truncate text-xs text-muted-foreground">{episode?.platform}</span>
							<Button type="button" variant="ghost" size="icon-sm" onclick={() => episode && confirmDeleteEpisode(episode)} aria-label={text.memoryDelete} title={text.memoryDelete}>
								<TrashIcon class="size-4" />
							</Button>
							<span class="ml-auto text-xs tabular-nums text-muted-foreground">{(episode?.occurredAt ?? '').slice(0, 16).replace('T', ' ')}</span>
						</div>
						{#if episode?.ingestionError}
							<p class="rounded-md border border-destructive/30 bg-destructive/10 px-2 py-1.5 text-xs break-words text-destructive">
								{episode.ingestionError}
							</p>
						{/if}
						{#if episode?.prompt}
							<div class="grid gap-1 rounded-md border bg-muted/20 px-2 py-2">
								<p class="text-xs font-medium text-muted-foreground">{text.sourceMessage}</p>
								<div class="memory-markdown text-sm leading-5">
									<SvelteMarkdown source={episode.prompt} />
								</div>
							</div>
						{:else}
							<p class="rounded-md border bg-muted/20 px-2 py-2 text-xs leading-5 text-muted-foreground">
								{text.sourceMessageUnavailable}
							</p>
						{/if}
					</article>
				{/if}
				{#each visibleFacts() as fact}
					<article class="grid gap-2 border-b px-3 py-3 last:border-b-0">
						<div class="flex min-w-0 flex-wrap items-center gap-2">
							<Badge variant="outline">{fact.sourceKind ?? text.source}</Badge>
							<span class="truncate text-xs text-muted-foreground">{scopeDisplayName(fact.scopeType, fact.namespaceID)}</span>
							{#if fact.validAt}
								<span class="text-xs tabular-nums text-muted-foreground">{fact.validAt.slice(0, 10)}</span>
							{/if}
							<span class="ml-auto text-xs tabular-nums text-muted-foreground">{factScoreText(fact.score)}</span>
						</div>
						<div class="memory-markdown text-sm leading-5">
							<SvelteMarkdown source={fact.content} />
						</div>
					</article>
				{/each}
				{#if visibleFacts().length === 0 && !selectedEpisode()}
					<p class="px-3 py-8 text-center text-xs text-muted-foreground">{text.noVisibleMemory}</p>
				{/if}
			</div>
		</aside>
	</section>
{/if}

{#if namespaces().length > 0}
	<section class="overflow-hidden rounded-lg border">
		{#each visibleNamespaces() as namespace}
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
		{#if namespaces().length > 10}
			<div class="flex justify-center border-t px-3 py-2">
				<Button type="button" variant="ghost" size="sm" onclick={() => (showAllNamespaces = !showAllNamespaces)}>
					{showAllNamespaces ? text.namespacesShowLess : namespacesShowAllText()}
				</Button>
			</div>
		{/if}
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
