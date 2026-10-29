<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
	let contentHeight = $state<number>();
</script>

<div
	class="-m-1 overflow-hidden transition-[height] duration-200 ease-out motion-reduce:transition-none"
	style:height={contentHeight === undefined ? 'auto' : `${contentHeight}px`}
>
	<div class="p-1" bind:offsetHeight={contentHeight}>{@render children()}</div>
</div>
