<script lang="ts">
	import OverviewDashboard from './overview-dashboard.svelte';
	import MonthCalendar from './month-calendar.svelte';
	import SideCharts from './side-charts.svelte';
	import DayDetailPanel from './day-detail-panel.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import AbsenceForm from '../absence-form.svelte';

	const attendance = getAttendanceState();
</script>

<div class="flex flex-col gap-4">
	<OverviewDashboard />
	<AbsenceForm />
	<div class="grid grid-cols-1 gap-4 lg:grid-cols-[1.8fr_1fr]">
		<MonthCalendar />
		<div class="flex flex-col gap-3">
			{#if attendance.selectedDate}
				<DayDetailPanel />
			{/if}
			<SideCharts />
		</div>
	</div>
</div>
