<script lang="ts">
	import type { Snippet } from 'svelte';
	import ActivityMarker from './activity-marker.svelte';

	let {
		activity,
		height = $bindable(0),
		children
	}: { activity: string; height?: number; children: Snippet } = $props();
</script>

{#snippet activityMarker()}
	{#if activity}
		<ActivityMarker label={activity} />
	{/if}
{/snippet}

<div bind:offsetHeight={height} class="absolute inset-x-0 bottom-0 z-10">
	<div class="relative">
		<div
			class="bg-background/60 pointer-events-none absolute inset-0 backdrop-blur-md [mask-image:linear-gradient(to_bottom,transparent,black_1rem)]"
		></div>
		<div class="to-background pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent from-30%"></div>
		<div class="relative">
			{#if activity}
				<div class="flex h-6 items-center px-4 sm:hidden">
					{@render activityMarker()}
				</div>
			{/if}
			{@render children()}
		</div>
	</div>
	<div class="bg-background hidden h-6 items-center px-4 pb-1 sm:flex">
		{@render activityMarker()}
	</div>
</div>
