<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import DeferredSection from '$lib/components/deferred-section.svelte';
	import LeaveBalanceSummary from '../leave/leave-balance-summary.svelte';
	import LeaveRequestDialog from '../leave/leave-request-dialog.svelte';
	import QuickActions from '../quick-actions.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import { buildDailyWorkTimeValues, buildWorkTimeChartLocations } from '../shared/work-time-chart-data';
	import { attendanceText } from '../text';
	import WorkStandardSummary from './work-standard-summary.svelte';

	type Props = {
		containerClass?: string;
	};

	let { containerClass = '' }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();

	const targetEmail = $derived(attendance.summary?.currentUserEmail || '');
	const chartEvents = $derived(
		(attendance.summary?.events ?? []).filter((event) => event.email === targetEmail)
	);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const dailyValues = $derived(buildDailyWorkTimeValues(
		attendance.summary?.month ?? '',
		chartEvents,
		{ currentDate: today, fallbackLocationName: text.location }
	));
	const chartLocations = $derived(buildWorkTimeChartLocations(attendance.summary?.locations ?? [], dailyValues));
</script>

<div
	class={`min-h-0 space-y-4 ${containerClass}`}
	data-testid="personal-tools-panel"
>
	{#if !attendance.summary}
		<AttendanceLoadingSkeleton rowCount={3} />
	{:else}
		<QuickActions />
		<DeferredSection>
			<WorkTimeChart
				title={text.myWorkTime}
				{dailyValues}
				locations={chartLocations}
				formatValue={(value) => formatHoursMinutes(value, text)}
				compact
			>
				{#snippet footer()}
					<WorkStandardSummary />
				{/snippet}
			</WorkTimeChart>
			{#snippet placeholder()}
				<Skeleton class="h-40 w-full rounded-xl" />
			{/snippet}
		</DeferredSection>
		<LeaveBalanceSummary />
		<LeaveRequestDialog />
	{/if}
</div>
