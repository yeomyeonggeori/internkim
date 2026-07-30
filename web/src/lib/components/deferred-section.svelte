<script lang="ts">
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	let { children, placeholder }: { children: Snippet; placeholder?: Snippet } = $props();

	let isReady = $state(false);

	onMount(() => {
		const frame = requestAnimationFrame(() => {
			isReady = true;
		});
		return () => cancelAnimationFrame(frame);
	});
</script>

{#if isReady}
	{@render children()}
{:else}
	{@render placeholder?.()}
{/if}
