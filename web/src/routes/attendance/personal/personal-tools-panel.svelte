<script lang="ts">
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import { getAttendanceState, type AttendanceLocation } from '../attendance-context.svelte';
	import AbsenceForm from '../absence-form.svelte';
	import QuickActions from '../quick-actions.svelte';
	import { eachDayOfMonth, todayDateInTimeZone } from '../shared/attendance-date';
	import { computeDayEvents } from '../shared/attendance-day-events';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import type { AttendanceWorkSegment } from '../shared/attendance-work-segments';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import type { DailyValue, WorkTimeChartLocation } from '../shared/work-time-chart-model';
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
	const dailyValues = $derived(buildDailyValues(attendance.summary?.month ?? '', chartEvents));
	const chartLocations = $derived(buildChartLocations(attendance.summary?.locations ?? [], dailyValues));

	function buildDailyValues(month: string, eventList: typeof chartEvents): DailyValue[] {
		if (!month) return [];
		return eachDayOfMonth(month).map((date) => {
			const day = computeDayEvents(date, eventList, { currentDate: today });
			return { date, minutesByLocation: sumSegmentMinutesByLocation(day.segments) };
		});
	}

	function sumSegmentMinutesByLocation(segments: AttendanceWorkSegment[]): Record<string, number> {
		const minutesByLocation: Record<string, number> = {};
		for (const segment of segments) {
			const locationKey = segment.locationName ?? segment.locationID ?? text.location;
			minutesByLocation[locationKey] = (minutesByLocation[locationKey] ?? 0) + segmentMinutes(segment);
		}
		return minutesByLocation;
	}

	function segmentMinutes(segment: AttendanceWorkSegment): number {
		if (!segment.isOpen) return segment.workedMinutes;
		const elapsed = (Date.now() - new Date(segment.clockIn.occurredAt).getTime()) / 60000;
		return Math.max(0, Math.round(elapsed));
	}

	function buildChartLocations(
		summaryLocations: AttendanceLocation[],
		dailyValueList: DailyValue[]
	): WorkTimeChartLocation[] {
		const chartLocations: WorkTimeChartLocation[] = summaryLocations.map((location) => ({
			key: location.name,
			name: location.name,
			color: location.color,
		}));
		const knownKeys = new Set(chartLocations.map((location) => location.key));
		for (const dailyValue of dailyValueList) {
			for (const locationKey of Object.keys(dailyValue.minutesByLocation)) {
				if (knownKeys.has(locationKey)) continue;
				knownKeys.add(locationKey);
				const locationByID = summaryLocations.find((location) => location.id === locationKey);
				chartLocations.push({
					key: locationKey,
					name: locationByID?.name ?? locationKey,
					color: locationByID?.color,
				});
			}
		}
		return chartLocations;
	}
</script>

<div
	class={`min-h-0 space-y-4 overflow-auto ${containerClass}`}
	data-testid="personal-tools-panel"
>
	<QuickActions />
	<WorkTimeChart
		title={text.myWorkTime}
		{dailyValues}
		locations={chartLocations}
		formatValue={(value) => formatHoursMinutes(value, text)}
		compact
	/>
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
</div>
