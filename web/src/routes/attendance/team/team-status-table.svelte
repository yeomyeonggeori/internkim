<script lang="ts">
	import { tick } from 'svelte';
	import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
	import type { FlowState } from '../../flow/flow-types';
	import type { AttendanceText } from '../text';
	import TeamStatusDateHeader from './team-status-date-header.svelte';
	import TeamStatusDayCell from './team-status-day-cell.svelte';
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
	};

	let { rows, statusDates, selectedDate, today, text }: Props = $props();
	let scrollContainer: HTMLDivElement | undefined = $state();
	let headerRow: HTMLDivElement | undefined = $state();
	let scrollContainerWidth = $state(0);
	let headerRowHeight = $state(0);
	let isDetailOpen = $state(false);
	let selectedDetailKey = $state<{ email: string; date: string } | null>(null);
	let calendarEvents = $state<CalendarEvent[]>([]);
	let flowState = $state<FlowState | null>(null);
	let isCalendarContextLoading = $state(false);
	let isFlowContextLoading = $state(false);
	let hasCalendarContextLoadFailed = $state(false);
	let hasFlowContextLoadFailed = $state(false);
	let openPersonHeaderEmail = $state<string | null>(null);
	let openDayTooltipKey = $state<string | null>(null);
	let lastScrollKey = $state('');
	let lastContextLoadKey = $state('');
	let lastContextLoadedAt = $state(0);
	let activeContextLoadKey = $state('');
	let contextRequestID = 0;

	const minimumMobileEmployeeColumnWidth = 8;
	const minimumEmployeeColumnWidth = 6;
	const maximumMobileEmployeeColumnWidth = 14;
	const maximumEmployeeColumnWidth = 13;
	const employeeColumnChromeWidth = 3;
	const locationLabelIconAllowance = 1.25;
	const dateColumnWidth = 4.5;
	const wideTableMinimumWidth = 640;
	const contextReloadTTLMilliseconds = 30_000;
	const minimumResponsiveEmployeeColumnWidth = $derived(
		scrollContainerWidth >= wideTableMinimumWidth ? minimumEmployeeColumnWidth : minimumMobileEmployeeColumnWidth
	);
	const maximumResponsiveEmployeeColumnWidth = $derived(
		scrollContainerWidth >= wideTableMinimumWidth ? maximumEmployeeColumnWidth : maximumMobileEmployeeColumnWidth
	);
	const employeeColumnWidth = $derived(calculateEmployeeColumnWidth(rows, minimumResponsiveEmployeeColumnWidth, maximumResponsiveEmployeeColumnWidth));
	const gridTemplateColumns = $derived(`${dateColumnWidth}rem repeat(${rows.length}, minmax(${employeeColumnWidth}rem, 1fr))`);
	const tableWidth = $derived(`${dateColumnWidth + rows.length * employeeColumnWidth}rem`);
	const selectedDetailBase = $derived.by(() => {
		if (!selectedDetailKey) return null;
		const selectedRow = rows.find((row) => row.email === selectedDetailKey?.email);
		const selectedDay = selectedRow?.days.find((day) => day.date === selectedDetailKey?.date);
		if (!selectedRow || !selectedDay) return null;
		return {
			displayName: selectedRow.displayName,
			email: selectedRow.email,
			image: selectedRow.image,
			mattermostUsername: selectedRow.mattermostUsername,
			day: selectedDay
		};
	});
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
		if (!headerRow) return;
		headerRowHeight = headerRow.getBoundingClientRect().height;
		const resizeObserver = new ResizeObserver((entries) => {
			headerRowHeight = entries[0]?.contentRect.height ?? headerRow?.getBoundingClientRect().height ?? 0;
		});
		resizeObserver.observe(headerRow);
		return () => resizeObserver.disconnect();
	});

	$effect(() => {
		const targetDate = selectedDate;
		const dates = statusDates.join(',');
		if (!scrollContainer || !targetDate || !dates) return;
		const scrollKey = `${targetDate}:${dates}`;
		if (lastScrollKey === scrollKey) return;
		lastScrollKey = scrollKey;
		tick().then(() => scrollToDateRow(targetDate));
	});

	$effect(() => {
		const firstDate = statusDates[0];
		const lastDate = statusDates.at(-1);
		if (!firstDate || !lastDate) return;
		void loadDayContextData(firstDate, lastDate, { force: false });
	});

	function calculateEmployeeColumnWidth(employeeRows: TeamStatusPersonRow[], minimumWidth: number, maximumWidth: number): number {
		const longestNameWidth = Math.max(0, ...employeeRows.map((row) => estimateTextWidth(row.displayName)));
		const longestLocationLabelWidth = Math.max(
			0,
			...employeeRows.map((row) => (row.currentLocationName ? estimateTextWidth(row.currentLocationName) + locationLabelIconAllowance : 0))
		);
		const longestContentWidth = Math.max(longestNameWidth, longestLocationLabelWidth);
		return clampWidth(longestContentWidth + employeeColumnChromeWidth, minimumWidth, maximumWidth);
	}

	function estimateTextWidth(textToMeasure: string): number {
		return Array.from(textToMeasure).reduce((width, character) => width + estimateCharacterWidth(character), 0);
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

	function setPersonHeaderOpen(email: string, isOpen: boolean): void {
		if (isOpen) {
			openPersonHeaderEmail = email;
			return;
		}
		if (openPersonHeaderEmail === email) openPersonHeaderEmail = null;
	}

	function setDayTooltipOpen(key: string, isOpen: boolean): void {
		if (isOpen) {
			openDayTooltipKey = key;
			return;
		}
		if (openDayTooltipKey === key) openDayTooltipKey = null;
	}

	function dateRowClass(date: string): string {
		if (date === today) return 'sticky z-[15] grid border-y bg-background shadow-md';
		return 'grid border-b last:border-b-0';
	}

	function openDayDetail(row: TeamStatusPersonRow, day: TeamStatusPersonDay): void {
		openDayTooltipKey = null;
		selectedDetailKey = { email: row.email, date: day.date };
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

	function scrollToDateRow(date: string): void {
		if (!scrollContainer) return;
		const dateRowHeader = scrollContainer.querySelector<HTMLElement>(`[data-testid="team-status-day-${date}"]`);
		if (!dateRowHeader) return;
		const containerRect = scrollContainer.getBoundingClientRect();
		const rowHeaderRect = dateRowHeader.getBoundingClientRect();
		const rowTop = rowHeaderRect.top - containerRect.top + scrollContainer.scrollTop;
		const targetCenter = rowTop + rowHeaderRect.height / 2;
		const viewportCenter = scrollContainer.clientHeight / 2;
		scrollContainer.scrollTop = Math.max(0, targetCenter - viewportCenter);
	}
</script>

<div bind:this={scrollContainer} class="h-full max-h-[calc(100vh-12rem)] max-w-full overflow-auto rounded-md border" data-testid="team-status-table">
	<div style:min-width={tableWidth} role="table" aria-label={text.teamMonthlyStatusTable}>
		<div bind:this={headerRow} class="sticky top-0 z-20 grid border-b bg-muted" style:grid-template-columns={gridTemplateColumns} role="row">
			<div class="sticky left-0 z-30 border-r bg-muted" role="columnheader"></div>
			{#each rows as row, employeeIndex (row.email)}
				<TeamStatusPersonHeader
					{row}
					columnIndex={employeeIndex}
					isLastColumn={employeeIndex === rows.length - 1}
					{text}
					isWorkTimeOpen={openPersonHeaderEmail === row.email}
					onWorkTimeOpenChange={(isOpen) => setPersonHeaderOpen(row.email, isOpen)}
				/>
			{/each}
		</div>
		<div role="rowgroup">
			{#if rows.length > 0}
				{#each statusDates as date, dateIndex (date)}
					<div
						class={dateRowClass(date)}
						style:grid-template-columns={gridTemplateColumns}
						style:top={date === today ? `${headerRowHeight}px` : undefined}
						style:bottom={date === today ? '0px' : undefined}
						role="row"
					>
						<TeamStatusDateHeader {date} {today} {text} />
						{#each rows as row, employeeIndex (row.email)}
							{@const dayTooltipKey = `${row.email}:${date}`}
							<TeamStatusDayCell
								day={row.days[dateIndex]}
								columnIndex={employeeIndex}
								isLastColumn={employeeIndex === rows.length - 1}
								personEmail={row.email}
								isWorkTooltipOpen={openDayTooltipKey === dayTooltipKey}
								onWorkTooltipOpenChange={(isOpen) => setDayTooltipOpen(dayTooltipKey, isOpen)}
								onOpenDayDetail={(selectedDay) => openDayDetail(row, selectedDay)}
							/>
						{/each}
					</div>
				{/each}
			{:else}
				<div class="py-8 text-center text-sm text-muted-foreground" role="row">
					<div role="cell">{text.noMembers}</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<TeamStatusDayDetailDialog {text} bind:isOpen={isDetailOpen} detail={selectedDetail} />
