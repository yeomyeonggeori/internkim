<script lang="ts">
	import { tick } from 'svelte';
	import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
	import type { FlowState } from '../../flow/flow-types';
	import type { AttendanceText } from '../text';
	import TeamStatusDateHeader from './team-status-date-header.svelte';
	import TeamStatusDayCell from './team-status-day-cell.svelte';
	import type { TeamStatusDayDetail } from './team-status-day-detail';
	import { buildTeamStatusDayContext } from './team-status-day-context';
	import { loadTeamStatusDayContextData } from './team-status-day-context-loader';
	import TeamStatusDayDetailDialog from './team-status-day-detail-dialog.svelte';
	import TeamStatusPersonHeader from './team-status-person-header.svelte';
	import type { TeamStatusPersonDay, TeamStatusPersonRow } from './team-status-table-model';

	type Props = {
		rows: TeamStatusPersonRow[];
		statusDates: string[];
		selectedDate: string;
		today: string;
		text: AttendanceText;
		onSelectDate: (date: string) => void;
	};

	let { rows, statusDates, selectedDate, today, text, onSelectDate }: Props = $props();
	let scrollContainer: HTMLDivElement | undefined = $state();
	let scrollContainerWidth = $state(0);
	let isDetailOpen = $state(false);
	let selectedDetailBase = $state<Omit<TeamStatusDayDetail, 'context'> | null>(null);
	let calendarEvents = $state<CalendarEvent[]>([]);
	let flowState = $state<FlowState | null>(null);
	let isCalendarContextLoading = $state(false);
	let isFlowContextLoading = $state(false);
	let hasCalendarContextLoadFailed = $state(false);
	let hasFlowContextLoadFailed = $state(false);
	let lastScrollKey = $state('');
	let lastContextLoadKey = $state('');
	let lastContextLoadedAt = $state(0);
	let activeContextLoadKey = $state('');
	let contextRequestID = 0;

	const minimumMobileEmployeeColumnWidth = 14;
	const minimumEmployeeColumnWidth = 7;
	const maximumMobileEmployeeColumnWidth = 14;
	const maximumEmployeeColumnWidth = 13;
	const employeeColumnChromeWidth = 2;
	const dayColumnWidth = 5.75;
	const wideTableMinimumWidth = 640;
	const contextReloadTTLMilliseconds = 30_000;
	const minimumResponsiveEmployeeColumnWidth = $derived(
		scrollContainerWidth >= wideTableMinimumWidth ? minimumEmployeeColumnWidth : minimumMobileEmployeeColumnWidth
	);
	const maximumResponsiveEmployeeColumnWidth = $derived(
		scrollContainerWidth >= wideTableMinimumWidth ? maximumEmployeeColumnWidth : maximumMobileEmployeeColumnWidth
	);
	const canShowCellTooltip = $derived(scrollContainerWidth >= wideTableMinimumWidth);
	const employeeColumnWidth = $derived(calculateEmployeeColumnWidth(rows, minimumResponsiveEmployeeColumnWidth, maximumResponsiveEmployeeColumnWidth));
	const gridTemplateColumns = $derived(`${employeeColumnWidth}rem repeat(${statusDates.length}, minmax(${dayColumnWidth}rem, ${dayColumnWidth}rem))`);
	const tableWidth = $derived(`${employeeColumnWidth + statusDates.length * dayColumnWidth}rem`);
	const selectedDetail = $derived(selectedDetailBase ? {
		...selectedDetailBase,
		context: buildTeamStatusDayContext(
			selectedDetailBase,
			selectedDetailBase.day.date,
			calendarEvents,
			flowState,
			text.dateLocale,
			text.calendarAllDay,
			{
				isCalendarEventsLoading: isCalendarContextLoading,
				isCompletedWorkLoading: isFlowContextLoading,
				hasCalendarEventsLoadFailed: hasCalendarContextLoadFailed,
				hasCompletedWorkLoadFailed: hasFlowContextLoadFailed
			}
		)
	} : null);

	$effect(() => {
		if (!scrollContainer) return;
		scrollContainerWidth = scrollContainer.clientWidth;
		const resizeObserver = new ResizeObserver((entries) => {
			scrollContainerWidth = entries[0]?.contentRect.width ?? scrollContainer?.clientWidth ?? 0;
		});
		resizeObserver.observe(scrollContainer);
		return () => resizeObserver.disconnect();
	});

	$effect(() => {
		const targetDate = selectedDate;
		const dates = statusDates.join(',');
		if (!scrollContainer || !targetDate || !dates) return;
		const scrollKey = `${targetDate}:${dates}`;
		if (lastScrollKey === scrollKey) return;
		lastScrollKey = scrollKey;
		tick().then(() => scrollToDate(targetDate));
	});

	$effect(() => {
		const firstDate = statusDates[0];
		const lastDate = statusDates.at(-1);
		if (!firstDate || !lastDate) return;
		void loadDayContextData(firstDate, lastDate, { force: false });
	});

	function calculateEmployeeColumnWidth(employeeRows: TeamStatusPersonRow[], minimumWidth: number, maximumWidth: number): number {
		const longestNameWidth = Math.max(0, ...employeeRows.map(row => estimateDisplayNameWidth(row.displayName)));
		return clampWidth(longestNameWidth + employeeColumnChromeWidth, minimumWidth, maximumWidth);
	}

	function estimateDisplayNameWidth(displayName: string): number {
		return Array.from(displayName).reduce((width, character) => width + estimateCharacterWidth(character), 0);
	}

	function estimateCharacterWidth(character: string): number {
		if (/[A-Z0-9]/.test(character)) return 0.6;
		if (/[a-z]/.test(character)) return 0.5;
		if (/\s/.test(character)) return 0.3;
		return 0.95;
	}

	function clampWidth(width: number, minimumWidth: number, maximumWidth: number): number {
		return Math.min(maximumWidth, Math.max(minimumWidth, width));
	}

	function openDayDetail(row: TeamStatusPersonRow, day: TeamStatusPersonDay): void {
		selectedDetailBase = {
			displayName: row.displayName,
			email: row.email,
			mattermostUsername: row.mattermostUsername,
			day
		};
		isDetailOpen = true;
		const firstDate = statusDates[0];
		const lastDate = statusDates.at(-1);
		if (!firstDate || !lastDate) return;
		void loadDayContextData(firstDate, lastDate, { force: true });
	}

	async function loadDayContextData(firstDate: string, lastDate: string, options: { force: boolean }): Promise<void> {
		const loadKey = `${firstDate}:${lastDate}`;
		if (activeContextLoadKey === loadKey) return;
		const hasLoadedContext = lastContextLoadKey === loadKey;
		const hasFreshSuccessfulContext = hasLoadedContext
			&& !hasCalendarContextLoadFailed
			&& !hasFlowContextLoadFailed
			&& Date.now() - lastContextLoadedAt <= contextReloadTTLMilliseconds;
		if (hasFreshSuccessfulContext) return;
		if (!options.force && hasLoadedContext) return;
		activeContextLoadKey = loadKey;
		const requestID = contextRequestID + 1;
		contextRequestID = requestID;
		isCalendarContextLoading = true;
		isFlowContextLoading = true;
		hasCalendarContextLoadFailed = false;
		hasFlowContextLoadFailed = false;
		const contextData = await loadTeamStatusDayContextData(firstDate, lastDate, text.loadFailed);
		if (requestID !== contextRequestID) return;
		calendarEvents = contextData.calendarEvents;
		flowState = contextData.flowState;
		hasCalendarContextLoadFailed = contextData.hasCalendarEventsLoadFailed;
		hasFlowContextLoadFailed = contextData.hasCompletedWorkLoadFailed;
		isCalendarContextLoading = false;
		isFlowContextLoading = false;
		lastContextLoadKey = loadKey;
		lastContextLoadedAt = contextData.hasCalendarEventsLoadFailed || contextData.hasCompletedWorkLoadFailed ? 0 : Date.now();
		activeContextLoadKey = '';
	}

	function scrollToDate(date: string): void {
		const index = statusDates.indexOf(date);
		if (!scrollContainer || index < 0) return;
		const remInPixels = Number.parseFloat(getComputedStyle(document.documentElement).fontSize);
		const employeeWidth = employeeColumnWidth * remInPixels;
		const columnWidth = dayColumnWidth * remInPixels;
		const targetCenter = employeeWidth + index * columnWidth + columnWidth / 2;
		const viewportCenter = scrollContainer.clientWidth / 2;
		scrollContainer.scrollLeft = Math.max(0, targetCenter - viewportCenter);
	}
</script>

<div bind:this={scrollContainer} class="h-full max-h-[calc(100vh-12rem)] max-w-full overflow-auto rounded-md border" data-testid="team-status-table">
	<div style:min-width={tableWidth} role="table" aria-label={text.teamMonthlyStatusTable}>
		<div role="rowgroup">
			<div class="sticky top-0 z-20 grid border-b bg-muted/30" style:grid-template-columns={gridTemplateColumns} role="row">
				<div class="sticky left-0 z-30 border-r bg-muted px-3 py-2 text-xs font-medium text-muted-foreground" role="columnheader">
					{text.teamMember}
				</div>
				{#each statusDates as date (date)}
					<TeamStatusDateHeader {date} {selectedDate} {today} {text} {onSelectDate} />
				{/each}
			</div>
		</div>
		<div role="rowgroup">
			{#each rows as row (row.email)}
				<div class="grid border-b last:border-b-0" style:grid-template-columns={gridTemplateColumns} role="row">
					<TeamStatusPersonHeader {row} />
					{#each row.days as day, index (day.date)}
						<TeamStatusDayCell
							{day}
							{index}
							personEmail={row.email}
							canShowTooltip={canShowCellTooltip}
							onOpenDayDetail={(selectedDay) => openDayDetail(row, selectedDay)}
						/>
					{/each}
				</div>
			{/each}
			{#if rows.length === 0}
				<div class="py-8 text-center text-sm text-muted-foreground" role="row">
					<div role="cell">{text.noMembers}</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<TeamStatusDayDetailDialog {text} bind:isOpen={isDetailOpen} detail={selectedDetail} />
