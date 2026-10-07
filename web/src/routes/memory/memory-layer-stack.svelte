<script lang="ts">
	import { cn } from '$lib/utils';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import type { MemoryLayer } from './memory-facts-api';
	import { memoryLayerDepth, memoryLayerKey } from './memory-workbench-model';
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
	<div class="flex flex-col gap-1 max-sm:sr-only">
		<h2 id="memory-layers-title" class="text-sm font-semibold">{text.layersTitle}</h2>
		<p class="text-xs text-muted-foreground">{text.layersRule}</p>
	</div>
	<UnderlineTabs.Root value={selectedKey} onValueChange={onSelect} class="sm:hidden">
		<UnderlineTabs.List>
			{@render layerTab('all', text.allLayers, total)}
			{#each layers as layer (memoryLayerKey(layer))}
				{@render layerTab(memoryLayerKey(layer), labelOf(layer), countOf(memoryLayerKey(layer)))}
			{/each}
		</UnderlineTabs.List>
	</UnderlineTabs.Root>
	<div class="flex flex-col gap-1.5 max-sm:hidden">
		{@render layerButton('all', text.allLayers, total)}
		<div class="flex flex-col">
			{#each layers as layer (memoryLayerKey(layer))}
				{@const depth = memoryLayerDepth[layer.scopeType]}
				<div class="relative py-0.75" style={`padding-left: ${depth * 16}px`}>
					{#each { length: depth } as _, level (level)}
						<span class="absolute inset-y-0 w-px bg-border" style={`left: ${level * 16 + 7}px`} aria-hidden="true"></span>
					{/each}
					{@render layerButton(memoryLayerKey(layer), labelOf(layer), countOf(memoryLayerKey(layer)))}
				</div>
			{/each}
		</div>
		<div class="flex flex-col gap-0.5 rounded-md border border-dashed px-3 py-2.5 text-xs text-muted-foreground">
			<span class="text-sm font-medium text-foreground">{text.recordFloor}</span>{text.recordFloorDescription}
		</div>
	</div>
</section>

{#snippet layerTab(key: string, label: string, count: number)}
	<UnderlineTabs.Trigger value={key}>{label}<span class="text-xs font-normal tabular-nums text-muted-foreground">{count}</span></UnderlineTabs.Trigger>
{/snippet}

{#snippet layerButton(key: string, label: string, count: number)}
	<button
		type="button"
		aria-pressed={selectedKey === key}
		class={cn('flex w-full min-w-0 items-center gap-2.5 rounded-md border px-3 py-2 text-left transition-colors hover:bg-muted/50 focus-visible:outline-2 focus-visible:outline-ring', selectedKey === key && 'border-foreground/40 bg-muted/60')}
		onclick={() => onSelect(key)}
	>
		<span class="min-w-0 flex-1 truncate text-sm font-medium">{label}</span>
		<span class="shrink-0 text-xs tabular-nums text-muted-foreground">{count}</span>
	</button>
{/snippet}
