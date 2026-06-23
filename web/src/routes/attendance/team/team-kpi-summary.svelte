<script lang="ts">
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import UserCheckIcon from '@lucide/svelte/icons/user-check';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import {
		teamThisMonthAggregate,
		teamThisWeekAggregate,
		teamTodayAggregate,
		uniquePeople,
	} from '../shared/attendance-aggregation';
	import { isWeekday, isoWeekStart, todayDateInTimeZone, utcDateKey } from '../shared/attendance-date';
	import KpiCardGrid, { type KpiItem } from '../shared/kpi-card-grid.svelte';
	import { attendanceText } from '../text';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const realtimeEvents = $derived(attendance.currentMonthSummary?.events ?? []);
	const realtimeAbsences = $derived(attendance.currentMonthSummary?.absences ?? []);
	const today = $derived(todayDateInTimeZone(attendance.currentMonthSummary?.timeZone));

	const todayAggregate = $derived(teamTodayAggregate(realtimeEvents, today));
	const weekAggregate = $derived(teamThisWeekAggregate(realtimeEvents, today));
	const monthAggregate = $derived(teamThisMonthAggregate(realtimeEvents, today));

	const teamSize = $derived(uniquePeople(realtimeEvents, realtimeAbsences).length);
	const weekdaysThisWeek = $derived(countWeekdays(isoWeekStart(today), today));
	const weekdaysThisMonth = $derived(countWeekdays(`${today.slice(0, 7)}-01`, today));

	const todayRate = $derived(teamSize ? Math.round((todayAggregate.workedPeople / teamSize) * 100) : 0);
	const weekRate = $derived(
		teamSize && weekdaysThisWeek ? Math.round((weekAggregate.workedDays / (teamSize * weekdaysThisWeek)) * 100) : 0
	);
	const monthRate = $derived(
		teamSize && weekdaysThisMonth ? Math.round((monthAggregate.workedDays / (teamSize * weekdaysThisMonth)) * 100) : 0
	);
	const statusIconClass = $derived(todayAggregate.workingNow > 0 ? 'text-success' : 'text-muted-foreground');

	const items = $derived<KpiItem[]>([
		{
			icon: UsersIcon,
			iconClass: statusIconClass,
			label: text.currentStatus,
			value: formatCountTemplate(text.peopleWorkingTemplate, todayAggregate.workingNow),
			sublabel: teamSize ? formatTodayClockIn(todayAggregate.workedPeople, teamSize) : undefined,
		},
		{
			icon: UserCheckIcon,
			label: text.todayAttendance,
			value: teamSize ? `${todayAggregate.workedPeople}/${formatCountTemplate(text.peopleCountTemplate, teamSize)}` : '-',
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

	function countWeekdays(start: string, end: string): number {
		if (!start || !end || start > end) return 0;
		let count = 0;
		const date = new Date(`${start}T00:00:00Z`);
		const endDate = new Date(`${end}T00:00:00Z`);
		while (date <= endDate) {
			const dateKey = utcDateKey(date);
			if (isWeekday(dateKey)) count += 1;
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
</script>

<KpiCardGrid {items} />
