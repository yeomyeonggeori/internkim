<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceMonthPicker from '../attendance-month-picker.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computeHeatmap } from '../shared/attendance-aggregation';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import {
		buildTeamAbsenceByDate,
		buildTeamCalendarCells,
		type TeamCalendarAbsence,
		type TeamCalendarDayCell
	} from './team-month-calendar-model';
	import TeamMonthDay from './team-month-day.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const heatCells = $derived(
		attendance.summary ? computeHeatmap(attendance.summary.month, attendance.summary.events, attendance.summary.absences) : []
	);
	const cells = $derived(attendance.summary ? buildTeamCalendarCells(heatCells, attendance.summary.month) : []);
	const absencesByDate = $derived(buildTeamAbsenceByDate(attendance.summary, text));
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const calendarMonth = $derived(attendance.summary?.month ?? attendance.selectedMonth);

	function selectMonth(month: string) {
		attendance.selectedMonth = month;
		attendance.selectedDate = '';
		attendance.load();
	}

	function selectDate(date: string) {
		if (!attendance.summary || !date.startsWith(attendance.summary.month)) return;
		attendance.selectedDate = date;
	}

	function countLabel(cell: TeamCalendarDayCell): string {
		return text.absenceCountTemplate
			.replace('{absence}', String(cell.absenceCount))
			.replace('{total}', String(cell.totalPeople));
	}

	function moreLabel(count: number): string {
		return text.moreAbsencesTemplate.replace('{count}', String(count));
	}

	function hiddenAbsenceCount(absences: TeamCalendarAbsence[]): number {
		return absences.filter((absence) => !absence.isVisible).length;
	}

	function weekdayLabels(): string[] {
		return [
			text.weekdaySunday,
			text.weekdayMonday,
			text.weekdayTuesday,
			text.weekdayWednesday,
			text.weekdayThursday,
			text.weekdayFriday,
			text.weekdaySaturday,
		];
	}
</script>

<Card.Root data-testid="team-month-calendar" class="h-full min-w-0">
	<Card.Header class="flex min-w-0 flex-col items-stretch gap-3 pb-2 sm:flex-row sm:items-start sm:justify-between">
		<Card.Title class="min-w-0 truncate pt-1 text-base">
			{text.teamAttendanceCalendarTitleTemplate.replace('{month}', calendarMonth)}
		</Card.Title>
		<div class="flex w-full shrink-0 justify-center sm:w-auto sm:justify-end">
			<AttendanceMonthPicker selectedMonth={calendarMonth} onSelectMonth={selectMonth} />
		</div>
	</Card.Header>
	<Card.Content class="min-w-0 max-w-full pt-0">
		<div class="grid w-full min-w-0 grid-cols-7 gap-x-0 gap-y-1 text-xs" data-testid="team-month-calendar-grid">
			{#each weekdayLabels() as label, index}
				<div class={`pb-1 text-center text-[11px] font-medium ${index === 0 || index === 6 ? 'text-destructive' : 'text-muted-foreground'}`}>
					{label}
				</div>
			{/each}
			{#each cells as cell (cell.date)}
				{@const absences = absencesByDate.get(cell.date) ?? []}
				<TeamMonthDay
					{cell}
					{absences}
					isSelected={attendance.selectedDate === cell.date}
					isToday={cell.date === today}
					countLabel={countLabel(cell)}
					moreLabel={moreLabel(hiddenAbsenceCount(absences))}
					onSelect={selectDate}
				/>
			{/each}
		</div>
	</Card.Content>
</Card.Root>
