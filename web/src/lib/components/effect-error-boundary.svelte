<script lang="ts">
	import type { Snippet } from 'svelte';
	import EffectErrorBoundaryReset from './effect-error-boundary-reset.svelte';

	let {
		children,
		failed: failureNotice,
		region,
		resetKey
	}: { children: Snippet; failed?: Snippet; region: string; resetKey?: string } = $props();

	function reportContainedError(error: unknown): void {
		console.error(`Contained an error in the ${region}; the rest of the app keeps running.`, error);
	}
</script>

<svelte:boundary onerror={reportContainedError}>
	{@render children()}
	{#snippet failed(_error, reset)}
		<EffectErrorBoundaryReset {resetKey} {reset} />
		{@render failureNotice?.()}
	{/snippet}
</svelte:boundary>
