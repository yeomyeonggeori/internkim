<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { getAttendanceState } from './attendance-context.svelte';
	import AbsenceForm from './absence-form.svelte';
	import DayDetailPanel from './personal/day-detail-panel.svelte';
	import MonthCalendar from './personal/month-calendar.svelte';
	import QuickActions from './quick-actions.svelte';
	import { eachDayOfMonth } from './shared/attendance-date';
	import { computeDayEvents } from './shared/attendance-day-events';
	import { formatHoursMinutes } from './shared/attendance-format';
	import WorkTimeChart from './shared/work-time-chart.svelte';
	import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();

	const targetEmail = $derived(attendance.summary?.currentUserEmail || '');
	const chartEvents = $derived(
		(attendance.summary?.events ?? []).filter((event) => event.email === targetEmail)
	);
	const dailyValues = $derived(buildDailyValues(attendance.summary?.month ?? '', chartEvents));

	function buildDailyValues(month: string, eventList: typeof chartEvents) {
		if (!month) return [];
		return eachDayOfMonth(month).map((date) => {
			const day = computeDayEvents(date, eventList);
			return { date, value: day.workedMinutes };
		});
	}
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<ClipboardCheckIcon class="size-4 text-muted-foreground" />
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-medium">{text.title}</p>
			<p class="truncate text-xs text-muted-foreground">{attendance.summary?.timeZone ?? '-'}</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => attendance.load()}>
			<RefreshCwIcon class={attendance.isLoading ? 'animate-spin' : ''} />
		</Button>
	</div>

	<div class="min-h-0 flex-1 space-y-4 overflow-auto p-4">
		<QuickActions />
		<WorkTimeChart title={text.myWorkTime} {dailyValues} formatValue={(value) => formatHoursMinutes(value, text)} compact />
		<MonthCalendar compact />
		{#if attendance.selectedDate}
			<DayDetailPanel compact />
		{/if}
		<AbsenceForm compact />
	</div>
</aside>
