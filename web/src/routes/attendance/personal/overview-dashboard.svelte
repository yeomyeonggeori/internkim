<script lang="ts">
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
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

	const attendance = getAttendanceState();

	const targetEmail = $derived(attendance.selectedEmail || attendance.summary?.currentUserEmail || '');

	const realtimeEvents = $derived(
		(attendance.currentMonthSummary?.events ?? []).filter((event) => event.email === targetEmail)
	);

	const chartEvents = $derived(
		(attendance.summary?.events ?? []).filter((event) => event.email === targetEmail)
	);

	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const statusLabel = $derived.by(() => {
		const status = statusForDay(today, realtimeEvents);
		if (status === 'working') return '근무 중';
		if (status === 'finished') return '퇴근';
		return '미출근';
	});

	const statusIconClass = $derived.by(() => {
		const status = statusForDay(today, realtimeEvents);
		if (status === 'working') return 'text-success';
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
			label: '현재 상태',
			value: statusLabel,
		},
		{
			icon: TimerIcon,
			label: '오늘',
			value: formatHoursMinutes(todayBucket.minutes),
			sublabel: todayBucket.inProgress ? '진행 중' : undefined,
		},
		{
			icon: TrendingUpIcon,
			label: '이번 주',
			value: formatHoursMinutes(weekBucket.minutes),
			sublabel: weekBucket.workedDays ? `${weekBucket.workedDays}일 근무` : undefined,
		},
		{
			icon: CalendarIcon,
			label: '이번 달',
			value: formatHoursMinutes(monthBucket.minutes),
			sublabel: monthBucket.workedDays ? `${monthBucket.workedDays}일 근무` : undefined,
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
	<WorkTimeChart title="내 근무 시간" {dailyValues} formatValue={formatHoursMinutes} />
</div>
