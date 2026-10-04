<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import { cn } from '$lib/utils';
	import type { MemoryLayer } from './memory-facts-api';
	import { memoryLayerDepth } from './memory-workbench-model';

	let { layer, class: className }: { layer: MemoryLayer; class?: string } = $props();
	const stripes = $derived(Array.from({ length: memoryLayerDepth[layer.scopeType] }, (_, index) => index));
</script>

<span class={cn('inline-flex shrink-0 gap-0.75', className)} aria-hidden="true">
	{#each stripes as stripe (stripe)}<ColorMarker />{/each}
</span>
