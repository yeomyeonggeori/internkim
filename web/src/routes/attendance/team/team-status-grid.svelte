<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceMonthPicker from '../attendance-month-picker.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { eachDayOfMonth, todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import { buildTeamStatusRows, resolveDefaultDate } from './team-status-table-model';
	import TeamStatusTable from './team-status-table.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	let searchText = $state('');

	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const calendarMonth = $derived(attendance.summary?.month ?? attendance.selectedMonth);
	const defaultAnchorDate = $derived(
		attendance.summary ? resolveDefaultDate(attendance.summary.month, attendance.summary.events, today) : today
	);
	const statusDates = $derived(attendance.summary ? eachDayOfMonth(attendance.summary.month) : []);
	const rows = $derived(
		attendance.summary
			? buildTeamStatusRows(
					attendance.summary.month,
					attendance.summary,
					text,
					today,
					attendance.currentMonthSummary ?? attendance.summary
				)
			: []
	);
	const filteredRows = $derived(filterRows(rows, searchText));

	function selectMonth(month: string) {
		attendance.selectedMonth = month;
		attendance.load();
	}

	function filterRows(rowsToFilter: typeof rows, query: string) {
		const normalizedQuery = query.trim().toLowerCase();
		if (!normalizedQuery) return rowsToFilter;
		return rowsToFilter.filter((row) => row.displayName.toLowerCase().includes(normalizedQuery));
	}
</script>

<Card.Root data-testid="team-status-grid" class="flex min-h-[calc(100vh-7rem)] min-w-0 flex-1 flex-col">
	<Card.Header class="flex min-w-0 flex-col gap-[10px] overflow-hidden pb-1 sm:flex-row sm:items-start sm:justify-between">
		<div class="flex w-full min-w-0 items-center justify-between gap-2 sm:w-auto">
			<Card.Title class="min-w-0 flex-1 truncate text-base sm:flex-none sm:whitespace-nowrap">{text.teamMonthlyStatus}</Card.Title>
			<div class="ml-auto min-w-0 max-w-full shrink-0 overflow-hidden sm:hidden">
				<AttendanceMonthPicker selectedMonth={calendarMonth} onSelectMonth={selectMonth} density="compact" />
			</div>
		</div>
		<div class="flex w-full min-w-0 flex-col gap-[10px] overflow-hidden sm:w-auto sm:flex-row sm:items-center sm:justify-end">
			<div class="hidden min-w-0 max-w-full overflow-hidden sm:block">
				<AttendanceMonthPicker selectedMonth={calendarMonth} onSelectMonth={selectMonth} />
			</div>
			<div class="relative w-full min-w-0 max-w-full sm:w-48">
				<SearchIcon class="pointer-events-none absolute left-2 top-2.5 size-4 text-muted-foreground" />
				<Input class="h-9 min-w-0 pl-8 focus-visible:border-input focus-visible:ring-0" placeholder={text.teamMemberSearchPlaceholder} bind:value={searchText} />
			</div>
		</div>
	</Card.Header>
	<Card.Content class="min-h-0 min-w-0 max-w-full flex-1 overflow-hidden">
		<TeamStatusTable
			rows={filteredRows}
			{statusDates}
			selectedDate={attendance.selectedDate || defaultAnchorDate}
			{today}
			{text}
		/>
	</Card.Content>
</Card.Root>
