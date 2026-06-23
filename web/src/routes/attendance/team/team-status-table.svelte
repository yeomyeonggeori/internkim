<script lang="ts">
	import { tick } from 'svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { AttendanceText } from '../text';
	import { isWeekend } from '../shared/attendance-date';
	import { absenceDisplayClass } from '../shared/color-tokens';
	import TeamStatusDayDetailDialog, { type TeamStatusDayDetail } from './team-status-day-detail-dialog.svelte';
	import type { TeamStatusPersonDay, TeamStatusPersonRow } from './team-status-table-model';

	type Props = {
		rows: TeamStatusPersonRow[];
		statusDates: string[];
		selectedDate: string;
		text: AttendanceText;
		onSelectDate: (date: string) => void;
	};

	let { rows, statusDates, selectedDate, text, onSelectDate }: Props = $props();
	let scrollContainer: HTMLDivElement | undefined = $state();
	let isDetailOpen = $state(false);
	let selectedDetail = $state<TeamStatusDayDetail | null>(null);
	let lastScrollKey = $state('');

	const employeeColumnWidth = 7;
	const dayColumnWidth = 5.75;
	const gridTemplateColumns = $derived(`${employeeColumnWidth}rem repeat(${statusDates.length}, minmax(${dayColumnWidth}rem, ${dayColumnWidth}rem))`);
	const tableWidth = $derived(`${employeeColumnWidth + statusDates.length * dayColumnWidth}rem`);

	$effect(() => {
		const targetDate = selectedDate;
		const dates = statusDates.join(',');
		if (!scrollContainer || !targetDate || !dates) return;
		const scrollKey = `${targetDate}:${dates}`;
		if (lastScrollKey === scrollKey) return;
		lastScrollKey = scrollKey;
		tick().then(() => scrollToDate(targetDate));
	});

	function dayHeaderParts(date: string): { weekday: string; dateLabel: string } {
		const day = new Date(`${date}T00:00:00Z`);
		return {
			weekday: weekdayLabel(day.getUTCDay()),
			dateLabel: `${day.getUTCMonth() + 1}/${day.getUTCDate()}`,
		};
	}

	function weekdayLabel(day: number): string {
		const labels = [
			text.weekdaySunday,
			text.weekdayMonday,
			text.weekdayTuesday,
			text.weekdayWednesday,
			text.weekdayThursday,
			text.weekdayFriday,
			text.weekdaySaturday,
		];
		return labels[day] ?? '';
	}

	function cellToneClass(day: TeamStatusPersonDay): string {
		if (day.tone === 'working') return 'bg-success/10 text-success';
		if (day.tone === 'finished') return 'bg-background text-foreground';
		if (day.tone === 'absence') return absenceDisplayClass(day.absenceTone ?? 'leave');
		if (day.tone === 'absent') return 'bg-destructive/10 text-destructive';
		return 'text-muted-foreground';
	}

	function cellButtonClass(day: TeamStatusPersonDay): string {
		const emptyClass = day.tone === 'empty'
			? 'h-11 min-w-12 hover:bg-muted/30'
			: 'h-11 min-w-16 hover:ring-1 hover:ring-border';
		return `inline-flex max-w-full flex-col items-center justify-center gap-1 rounded-sm px-3 text-xs font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${emptyClass} ${cellToneClass(day)}`;
	}

	function cellTitle(day: TeamStatusPersonDay): string {
		if (day.locationName && day.locationName !== day.label) return `${day.label} · ${day.locationName}`;
		return day.label;
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

<div bind:this={scrollContainer} class="h-full max-h-[24rem] max-w-full overflow-auto rounded-md border xl:max-h-none" data-testid="team-status-table">
	<div style:min-width={tableWidth} role="table" aria-label={text.teamMonthlyStatusTable}>
		<div role="rowgroup">
			<div class="sticky top-0 z-20 grid border-b bg-muted/30" style:grid-template-columns={gridTemplateColumns} role="row">
				<div class="sticky left-0 z-30 border-r bg-muted px-2 py-2 text-xs font-medium text-muted-foreground" role="columnheader">
					{text.teamMember}
				</div>
				{#each statusDates as date (date)}
					{@const header = dayHeaderParts(date)}
					<div class="p-1 text-center" role="columnheader">
						<button
							type="button"
							class={`flex h-8 w-full flex-col items-center justify-center rounded-sm border-b-2 text-xs transition hover:bg-background ${
								selectedDate === date ? 'border-primary bg-background text-foreground' : 'border-transparent text-muted-foreground'
							}`}
							data-testid={`team-status-day-${date}`}
							onclick={() => onSelectDate(date)}
						>
							<span class={`font-semibold ${isWeekend(date) ? 'text-destructive' : ''}`}>{header.weekday}</span>
							<span class="font-medium tabular-nums">{header.dateLabel}</span>
						</button>
					</div>
				{/each}
			</div>
		</div>
		<div role="rowgroup">
			{#each rows as row (row.email)}
				<div class="grid border-b last:border-b-0" style:grid-template-columns={gridTemplateColumns} role="row">
					<div class="sticky left-0 z-10 min-w-0 border-r bg-card px-2 py-2" role="rowheader">
						<div class="flex min-w-0 items-center gap-2">
							<PersonAvatar name={row.displayName} email={row.email} seed={row.email || row.mattermostUsername || row.displayName} class="size-6 shrink-0" />
							<div class="min-w-0 truncate text-sm font-medium">{row.displayName}</div>
						</div>
					</div>
					{#each row.days as day, index (day.date)}
						<div class={`flex items-center justify-center p-1.5 text-center ${index === 0 ? '' : 'border-l'}`} role="cell">
							<button
								type="button"
								class={cellButtonClass(day)}
								title={cellTitle(day)}
								data-testid={`team-status-cell-${row.email}-${day.date}`}
								onclick={() => openDayDetail(row, day)}
							>
								<span class="flex max-w-full items-center gap-1" style:color={day.locationColor}>
									{#if day.locationName}
										<span
											class={`size-1.5 shrink-0 rounded-full ${day.locationColor ? '' : 'bg-success'}`}
											style:background-color={day.locationColor}
										></span>
									{/if}
									<span class="min-w-0 truncate">{day.label}</span>
								</span>
								{#if day.detailLabel}
									<span class="max-w-full truncate text-[10px] opacity-70" style:color={day.locationColor}>{day.detailLabel}</span>
								{/if}
							</button>
						</div>
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
