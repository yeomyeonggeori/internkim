<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children, region }: { children: Snippet; region: string } = $props();

	function reportContainedError(error: unknown): void {
		console.error(`Contained an error in the ${region}; the rest of the app keeps running.`, error);
	}
</script>

<svelte:boundary onerror={reportContainedError}>
	{@render children()}
</svelte:boundary>
