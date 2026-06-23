<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { createPageText } from '$lib/i18n/page-text.svelte';
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
		attendance.summary ? buildTeamStatusRows(attendance.summary.month, attendance.summary, text) : []
	);
	const filteredRows = $derived(filterRows(rows, searchText));

	function selectDate(date: string) {
		attendance.selectedDate = date;
	}

	function filterRows(rowsToFilter: typeof rows, query: string) {
		const normalizedQuery = query.trim().toLowerCase();
		if (!normalizedQuery) return rowsToFilter;
		return rowsToFilter.filter((row) => row.displayName.toLowerCase().includes(normalizedQuery));
	}
</script>

<Card.Root data-testid="team-status-grid" class="flex min-h-0 min-w-0 flex-1 flex-col">
	<Card.Header class="flex min-w-0 flex-col gap-3 pb-3 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<Card.Title class="text-base">{text.teamMonthlyStatus}</Card.Title>
			<Card.Description>{calendarMonth}</Card.Description>
		</div>
		<div class="relative w-full sm:w-48">
			<SearchIcon class="pointer-events-none absolute left-2 top-2.5 size-4 text-muted-foreground" />
			<Input class="h-9 pl-8" placeholder={text.teamMemberSearchPlaceholder} bind:value={searchText} />
		</div>
	</Card.Header>
	<Card.Content class="min-h-0 min-w-0 max-w-full flex-1 overflow-hidden">
		<TeamStatusTable
			rows={filteredRows}
			{statusDates}
			selectedDate={attendance.selectedDate || defaultAnchorDate}
			{text}
			onSelectDate={selectDate}
		/>
	</Card.Content>
</Card.Root>
