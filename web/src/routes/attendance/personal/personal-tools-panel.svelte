<script lang="ts">
	import { buttonVariants } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Dialog from '$lib/components/ui/dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import { getAttendanceState } from '../attendance-context.svelte';
	import AbsenceForm from '../absence-form.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import DeferredSection from '$lib/components/deferred-section.svelte';
	import QuickActions from '../quick-actions.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import { buildDailyWorkTimeValues, buildWorkTimeChartLocations } from '../shared/work-time-chart-data';
	import { attendanceText } from '../text';

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
	class={`min-h-0 space-y-4 overflow-auto ${containerClass}`}
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
			/>
			{#snippet placeholder()}
				<Skeleton class="h-40 w-full rounded-xl" />
			{/snippet}
		</DeferredSection>
		<Dialog.Root>
			<Dialog.Trigger class={cn(buttonVariants({ variant: 'outline' }), 'w-full')}>
				{text.absenceFormTitle}
			</Dialog.Trigger>
			<Dialog.Content class="max-w-md">
				<Dialog.Header>
					<Dialog.Title>{text.absenceFormTitle}</Dialog.Title>
				</Dialog.Header>
				<AbsenceForm />
			</Dialog.Content>
		</Dialog.Root>
	{/if}
</div>
