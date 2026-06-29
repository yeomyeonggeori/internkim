<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import AbsenceForm from '../absence-form.svelte';
	import QuickActions from '../quick-actions.svelte';
	import { eachDayOfMonth, todayDateInTimeZone } from '../shared/attendance-date';
	import { computeDayEvents } from '../shared/attendance-day-events';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import { attendanceText } from '../text';
	import DayDetailPanel from './day-detail-panel.svelte';
	import MonthCalendar from './month-calendar.svelte';

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

	function buildDailyValues(month: string, eventList: typeof chartEvents) {
		if (!month) return [];
		return eachDayOfMonth(month).map((date) => {
			const day = computeDayEvents(date, eventList, { currentDate: today });
			return { date, value: day.workedMinutes };
		});
	}
</script>

<div
	class={`min-h-0 space-y-4 overflow-auto ${containerClass}`}
	data-testid="personal-tools-panel"
>
	<QuickActions />
	<WorkTimeChart title={text.myWorkTime} {dailyValues} formatValue={(value) => formatHoursMinutes(value, text)} compact />
	<MonthCalendar compact />
	{#if attendance.selectedDate}
		<DayDetailPanel compact />
	{/if}
	<AbsenceForm compact />
</div>
