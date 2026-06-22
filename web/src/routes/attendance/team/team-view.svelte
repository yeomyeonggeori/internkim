<script lang="ts">
	import TeamKpiSummary from './team-kpi-summary.svelte';
	import TeamMonthCalendar from './team-month-calendar.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';
</script>

<div class="flex min-w-0 flex-col gap-4">
	<div data-testid="team-top-dashboard" class="grid min-w-0 grid-cols-1 items-stretch gap-4 xl:grid-cols-2">
		<div class="flex min-h-0 min-w-0 flex-col gap-4">
			<TeamKpiSummary />
			<TeamStatusGrid />
		</div>
		<TeamMonthCalendar />
	</div>
</div>
