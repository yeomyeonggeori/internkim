<script lang="ts">
	import OverviewDashboard from './overview-dashboard.svelte';
	import StatusBoard from './status-board.svelte';
	import MonthHeatmap from './month-heatmap.svelte';
</script>

<div class="flex flex-col gap-4">
	<OverviewDashboard />
	<div class="grid grid-cols-1 gap-4 lg:grid-cols-[1.6fr_1fr]">
		<StatusBoard />
		<MonthHeatmap />
	</div>
</div>
