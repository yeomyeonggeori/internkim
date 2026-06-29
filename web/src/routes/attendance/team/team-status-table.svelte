<script lang="ts">
	import { tick } from 'svelte';
	import type { AttendanceText } from '../text';
	import TeamStatusDateHeader from './team-status-date-header.svelte';
	import TeamStatusDayCell from './team-status-day-cell.svelte';
	import type { TeamStatusDayDetail } from './team-status-day-detail';
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
	let selectedDetail = $state<TeamStatusDayDetail | null>(null);
	let lastScrollKey = $state('');

	const minimumMobileEmployeeColumnWidth = 5;
	const minimumEmployeeColumnWidth = 7;
	const maximumMobileEmployeeColumnWidth = 7;
	const maximumEmployeeColumnWidth = 13;
	const employeeColumnChromeWidth = 2;
	const dayColumnWidth = 5.75;
	const wideTableMinimumWidth = 640;
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
		selectedDetail = {
			displayName: row.displayName,
			email: row.email,
			day,
		};
		isDetailOpen = true;
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
