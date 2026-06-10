<script lang="ts">
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import KpiCardGrid, { type KpiItem } from '../shared/kpi-card-grid.svelte';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import {
		computeDayEvents,
		groupEventsByDay,
		statusForDay,
		thisMonthMinutes,
		thisWeekMinutes,
		todayMinutes,
	} from '../shared/attendance-aggregation';
	import { eachDayOfMonth } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const targetEmail = $derived(attendance.summary?.currentUserEmail || '');

	const realtimeEvents = $derived(
		(attendance.currentMonthSummary?.events ?? []).filter((event) => event.email === targetEmail)
	);
	const realtimeAbsences = $derived(
		(attendance.currentMonthSummary?.absences ?? []).filter((absence) => absence.email === targetEmail)
	);

	const chartEvents = $derived(
		(attendance.summary?.events ?? []).filter((event) => event.email === targetEmail)
	);

	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const statusLabel = $derived.by(() => {
		const status = statusForDay(today, realtimeEvents, realtimeAbsences);
		if (status === 'working') return text.working;
		if (status === 'finished') return text.finished;
		if (status === 'absence') return text.absence;
		return text.absent;
	});

	const statusIconClass = $derived.by(() => {
		const status = statusForDay(today, realtimeEvents, realtimeAbsences);
		if (status === 'working') return 'text-success';
		if (status === 'absence') return 'text-info';
		if (status === 'finished') return 'text-muted-foreground';
		return 'text-muted-foreground';
	});

	const todayBucket = $derived(todayMinutes(realtimeEvents, today));
	const weekBucket = $derived(thisWeekMinutes(realtimeEvents, today));
	const monthBucket = $derived(thisMonthMinutes(realtimeEvents, today));

	const items = $derived<KpiItem[]>([
		{
			icon: ActivityIcon,
			iconClass: statusIconClass,
			label: text.currentStatus,
			value: statusLabel,
		},
		{
			icon: TimerIcon,
			label: text.today,
			value: formatHoursMinutes(todayBucket.minutes),
			sublabel: todayBucket.inProgress ? text.inProgress : undefined,
		},
		{
			icon: TrendingUpIcon,
			label: text.thisWeek,
			value: formatHoursMinutes(weekBucket.minutes),
			sublabel: weekBucket.workedDays ? text.workedDayTemplate.replace('{count}', String(weekBucket.workedDays)) : undefined,
		},
		{
			icon: CalendarIcon,
			label: text.thisMonth,
			value: formatHoursMinutes(monthBucket.minutes),
			sublabel: monthBucket.workedDays ? text.workedDayTemplate.replace('{count}', String(monthBucket.workedDays)) : undefined,
		},
	]);

	const dailyValues = $derived(buildDailyValues(attendance.summary?.month ?? '', chartEvents));

	function buildDailyValues(month: string, eventList: typeof chartEvents) {
		if (!month) return [];
		const grouped = groupEventsByDay(eventList);
		return eachDayOfMonth(month).map((date) => {
			const day = computeDayEvents(date, grouped.get(date) ?? []);
			return { date, value: day.workedMinutes };
		});
	}
</script>

<div class="flex flex-col gap-4">
	<KpiCardGrid {items} />
	<WorkTimeChart title={text.myWorkTime} {dailyValues} formatValue={formatHoursMinutes} />
</div>
