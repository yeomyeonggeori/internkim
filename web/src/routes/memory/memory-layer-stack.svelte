<script lang="ts">
	import { cn } from '$lib/utils';
	import type { MemoryLayer } from './memory-facts-api';
	import MemoryLayerDepth from './memory-layer-depth.svelte';
	import { memoryLayerKey } from './memory-workbench-model';
	import type { MemoryText } from './text';

	type Props = {
		layers: MemoryLayer[];
		selectedKey: string;
		countOf: (key: string) => number;
		labelOf: (layer: MemoryLayer) => string;
		onSelect: (key: string) => void;
		text: MemoryText;
	};

	let { layers, selectedKey, countOf, labelOf, onSelect, text }: Props = $props();
	const total = $derived(layers.reduce((sum, layer) => sum + countOf(memoryLayerKey(layer)), 0));
</script>

<section class="flex flex-col gap-3" aria-labelledby="memory-layers-title">
	<div class="flex flex-col gap-1">
		<h2 id="memory-layers-title" class="text-sm font-semibold">{text.layersTitle}</h2>
		<p class="text-xs text-muted-foreground">{text.layersRule}</p>
	</div>
	<div class="flex flex-col gap-1.5">
		{@render layerButton('all', text.allLayers, total, undefined)}
		{#each layers as layer (memoryLayerKey(layer))}
			{@render layerButton(memoryLayerKey(layer), labelOf(layer), countOf(memoryLayerKey(layer)), layer)}
		{/each}
		<div class="flex flex-col gap-0.5 rounded-md border border-dashed px-3 py-2.5 text-xs text-muted-foreground">
			<span class="text-sm font-medium text-foreground">{text.recordFloor}</span>{text.recordFloorDescription}
		</div>
	</div>
</section>

{#snippet layerButton(key: string, label: string, count: number, depthOf: MemoryLayer | undefined)}
	<button
		type="button"
		aria-pressed={selectedKey === key}
		class={cn('flex min-w-0 items-center gap-2.5 rounded-md border px-3 py-2 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring', selectedKey === key && 'border-foreground/40 bg-muted/60')}
		onclick={() => onSelect(key)}
	>
		{#if depthOf}<MemoryLayerDepth layer={depthOf} />{/if}
		<span class="min-w-0 flex-1 truncate text-sm font-medium">{label}</span>
		<span class="shrink-0 text-xs tabular-nums text-muted-foreground">{count}</span>
	</button>
{/snippet}
