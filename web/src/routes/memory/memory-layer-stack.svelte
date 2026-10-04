<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import { cn } from '$lib/utils';
	import type { MemoryLayer } from './memory-facts-api';
	import { memoryLayerKey, memoryLayerTones } from './memory-workbench-model';
	import type { MemoryText } from './text';

	type Props = {
		layers: MemoryLayer[];
		selectedKey: string;
		countOf: (key: string) => number;
		labelOf: (layer: MemoryLayer) => string;
		readerOf: (layer: MemoryLayer) => string;
		onSelect: (key: string) => void;
		text: MemoryText;
	};

	let { layers, selectedKey, countOf, labelOf, readerOf, onSelect, text }: Props = $props();
	const depthIndent: Record<MemoryLayer['scopeType'], string> = { person: '', circle: 'ml-2', workspace: 'ml-4' };
	const total = $derived(layers.reduce((sum, layer) => sum + countOf(memoryLayerKey(layer)), 0));
</script>

<section class="flex flex-col gap-3" aria-labelledby="memory-layers-title">
	<div class="flex flex-col gap-1">
		<h2 id="memory-layers-title" class="text-sm font-semibold">{text.layersTitle}</h2>
		<p class="text-xs text-muted-foreground">{text.layersRule}</p>
	</div>
	<div class="flex flex-col gap-1.5">
		{@render layerButton('all', text.allLayers, text.allLayersReader, total, 'bg-foreground', '')}
		{#each layers as layer (memoryLayerKey(layer))}
			{@render layerButton(memoryLayerKey(layer), labelOf(layer), readerOf(layer), countOf(memoryLayerKey(layer)), memoryLayerTones[layer.scopeType], depthIndent[layer.scopeType])}
		{/each}
		<div class="ml-6 flex gap-2.5 rounded-md border border-dashed px-3 py-2.5 text-xs text-muted-foreground">
			<span class="mt-0.5 h-3 w-0.75 shrink-0 rounded-full border border-dashed border-muted-foreground" aria-hidden="true"></span>
			<span class="flex min-w-0 flex-col gap-0.5"><span class="text-sm font-medium text-foreground">{text.recordFloor}</span>{text.recordFloorDescription}</span>
		</div>
	</div>
</section>

{#snippet layerButton(key: string, label: string, reader: string, count: number, tone: string, indent: string)}
	<button
		type="button"
		aria-pressed={selectedKey === key}
		class={cn('flex min-w-0 items-center gap-2.5 rounded-md border px-3 py-2 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring', indent, selectedKey === key && 'border-foreground/40 bg-muted/60')}
		onclick={() => onSelect(key)}
	>
		<ColorMarker class={tone} />
		<span class="flex min-w-0 flex-1 flex-col"><span class="truncate text-sm font-medium">{label}</span><span class="truncate text-xs text-muted-foreground">{reader}</span></span>
		<span class="shrink-0 text-xs tabular-nums text-muted-foreground">{count}</span>
	</button>
{/snippet}
