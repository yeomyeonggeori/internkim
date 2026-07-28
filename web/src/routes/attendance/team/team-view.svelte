<script lang="ts">
	import { onMount } from 'svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';

	let isGridMounted = $state(false);

	onMount(() => {
		const frame = requestAnimationFrame(() => {
			isGridMounted = true;
		});
		return () => cancelAnimationFrame(frame);
	});
</script>

<div class="flex min-h-0 min-w-0 flex-col">
	{#if isGridMounted}
		<TeamStatusGrid />
	{:else}
		<AttendanceLoadingSkeleton rowCount={8} />
	{/if}
</div>
