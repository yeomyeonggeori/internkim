<script lang="ts">
	import UsersIcon from '@lucide/svelte/icons/users';
	import UserCheckIcon from '@lucide/svelte/icons/user-check';
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone, utcDateKey } from '../shared/attendance-date';
	import KpiCardGrid, { type KpiItem } from '../shared/kpi-card-grid.svelte';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import {
		computeDayEvents,
		groupEventsByDay,
		teamThisMonthAggregate,
		teamThisWeekAggregate,
		teamTodayAggregate,
		uniquePeople,
	} from '../shared/attendance-aggregation';
	import { eachDayOfMonth, isWeekday, isoWeekStart } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const realtimeEvents = $derived(attendance.currentMonthSummary?.events ?? []);
	const chartEvents = $derived(attendance.summary?.events ?? []);

	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const todayBucket = $derived(teamTodayAggregate(realtimeEvents, today));
	const weekBucket = $derived(teamThisWeekAggregate(realtimeEvents, today));
	const monthBucket = $derived(teamThisMonthAggregate(realtimeEvents, today));

	const teamSize = $derived(uniquePeople(realtimeEvents).length);
	const weekdaysThisWeek = $derived(weekdaysBetween(isoWeekStart(today), today));
	const weekdaysThisMonth = $derived(weekdaysBetween(`${today.slice(0, 7)}-01`, today));

	const todayRate = $derived(teamSize ? Math.round((todayBucket.workedPeople / teamSize) * 100) : 0);
	const weekRate = $derived(
		teamSize && weekdaysThisWeek ? Math.round((weekBucket.workedDays / (teamSize * weekdaysThisWeek)) * 100) : 0
	);
	const monthRate = $derived(
		teamSize && weekdaysThisMonth ? Math.round((monthBucket.workedDays / (teamSize * weekdaysThisMonth)) * 100) : 0
	);

	const statusIconClass = $derived(todayBucket.workingNow > 0 ? 'text-success' : 'text-muted-foreground');

	function weekdaysBetween(start: string, end: string): number {
		if (!start || !end || start > end) return 0;
		let count = 0;
		const date = new Date(`${start}T00:00:00Z`);
		const endDate = new Date(`${end}T00:00:00Z`);
		while (date <= endDate) {
			const iso = utcDateKey(date);
			if (isWeekday(iso)) count += 1;
			date.setUTCDate(date.getUTCDate() + 1);
		}
		return count;
	}

	function formatCountTemplate(template: string, count: number): string {
		return template.replace('{count}', String(count));
	}

	function formatTodayClockIn(worked: number, total: number): string {
		return text.todayClockInTemplate.replace('{worked}', String(worked)).replace('{total}', String(total));
	}

	const items = $derived<KpiItem[]>([
		{
			icon: UsersIcon,
			iconClass: statusIconClass,
			label: text.currentStatus,
			value: formatCountTemplate(text.peopleWorkingTemplate, todayBucket.workingNow),
			sublabel: teamSize ? formatTodayClockIn(todayBucket.workedPeople, teamSize) : undefined,
		},
		{
			icon: UserCheckIcon,
			label: text.todayAttendance,
			value: teamSize ? `${todayBucket.workedPeople}/${formatCountTemplate(text.peopleCountTemplate, teamSize)}` : '-',
			sublabel: teamSize ? `${todayRate}%` : undefined,
		},
		{
			icon: TrendingUpIcon,
			label: text.thisWeekAverage,
			value: teamSize ? `${weekRate}%` : '-',
			sublabel: weekdaysThisWeek ? formatCountTemplate(text.dayBasisTemplate, weekdaysThisWeek) : undefined,
		},
		{
			icon: CalendarIcon,
			label: text.thisMonthAverage,
			value: teamSize ? `${monthRate}%` : '-',
			sublabel: weekdaysThisMonth ? formatCountTemplate(text.dayBasisTemplate, weekdaysThisMonth) : undefined,
		},
	]);

	const dailyValues = $derived(buildTeamDailyValues(attendance.summary?.month ?? '', chartEvents));

	function buildTeamDailyValues(month: string, eventList: typeof chartEvents) {
		if (!month) return [];
		const grouped = groupEventsByDay(eventList);
		return eachDayOfMonth(month).map((date) => {
			const dayEvents = grouped.get(date) ?? [];
			const byEmail = new Map<string, typeof eventList>();
			for (const event of dayEvents) {
				const list = byEmail.get(event.email) ?? [];
				list.push(event);
				byEmail.set(event.email, list);
			}
			let total = 0;
			for (const [, personEvents] of byEmail) {
				total += computeDayEvents(date, personEvents).workedMinutes;
			}
			return { date, value: total };
		});
	}
</script>

<div class="flex flex-col gap-4">
	<KpiCardGrid {items} />
	<WorkTimeChart title={text.teamWorkTime} {dailyValues} formatValue={formatHoursMinutes} />
</div>
