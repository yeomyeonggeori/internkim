<script lang="ts">
	import FilterCombobox, { type FilterComboboxOption } from '$lib/components/filter-combobox.svelte';
	import * as Card from '$lib/components/ui/card';
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
	const memberOptions = $derived<FilterComboboxOption[]>(rows.map((row) => ({ value: row.displayName, label: row.displayName })));

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
			<FilterCombobox
				bind:value={searchText}
				options={memberOptions}
				label={text.teamMemberSelectLabel}
				searchPlaceholder={text.teamMemberSearchPlaceholder}
				class="w-full min-w-0 sm:w-48"
			/>
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
