<script lang="ts">
	import DeferredSection from '$lib/components/deferred-section.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';
</script>

<div class="flex min-h-0 min-w-0 flex-col">
	<DeferredSection>
		<TeamStatusGrid />
		{#snippet placeholder()}
			<AttendanceLoadingSkeleton rowCount={8} />
		{/snippet}
	</DeferredSection>
</div>
