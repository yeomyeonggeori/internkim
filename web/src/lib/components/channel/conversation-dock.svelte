<script lang="ts">
	import type { Snippet } from 'svelte';
	import ActivityMarker from './activity-marker.svelte';

	let {
		activity,
		height = $bindable(0),
		children
	}: { activity: string; height?: number; children: Snippet } = $props();
</script>

<div bind:offsetHeight={height} class="pointer-events-none absolute inset-x-0 bottom-0 z-10 [&>*]:pointer-events-auto">
	{#if activity}
		<div class="absolute inset-x-0 bottom-full flex h-6 items-center px-4">
			<ActivityMarker label={activity} />
		</div>
	{/if}
	{@render children()}
</div>
