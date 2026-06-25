<script lang="ts">
	import { tick } from 'svelte';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { AttendanceText } from '../text';
	import { isWeekend } from '../shared/attendance-date';
	import TeamStatusDayDetailDialog, { type TeamStatusDayDetail } from './team-status-day-detail-dialog.svelte';
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
	let isDetailOpen = $state(false);
	let selectedDetail = $state<TeamStatusDayDetail | null>(null);
	let lastScrollKey = $state('');

	const minimumEmployeeColumnWidth = 7;
	const maximumEmployeeColumnWidth = 13;
	const employeeColumnChromeWidth = 3.75;
	const dayColumnWidth = 5.75;
	const employeeColumnWidth = $derived(calculateEmployeeColumnWidth(rows));
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

	function dayHeaderButtonClass(date: string): string {
		if (date === today) return 'border-primary bg-primary text-primary-foreground hover:bg-primary/90';
		if (selectedDate === date) return 'border-primary bg-sky-50 text-foreground hover:bg-sky-50 dark:bg-sky-950/30';
		return 'border-transparent bg-transparent text-muted-foreground hover:bg-background';
	}

	function dayHeaderWeekdayClass(date: string): string {
		if (date === today) return 'text-primary-foreground';
		if (isWeekend(date)) return 'text-destructive';
		return '';
	}

	function cellToneClass(day: TeamStatusPersonDay): string {
		if (day.tone === 'working') return 'bg-background text-foreground';
		if (day.tone === 'finished') return 'bg-background text-foreground';
		if (day.tone === 'absence') return absenceBackgroundClass(day);
		if (day.tone === 'absent') return 'bg-background text-destructive';
		return 'text-muted-foreground';
	}

	function absenceBackgroundClass(day: TeamStatusPersonDay): string {
		if (day.absenceTone === 'other') return 'bg-background text-foreground';
		return 'bg-[color-mix(in_oklab,var(--color-info)_8%,var(--color-background))] text-foreground';
	}

	function cellButtonClass(day: TeamStatusPersonDay): string {
		const emptyClass = day.tone === 'empty'
			? 'hover:bg-muted/30'
			: 'hover:bg-muted/30';
		return `flex h-full min-h-14 w-full max-w-none flex-col items-center justify-center gap-1 px-1 text-xs font-medium transition focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${emptyClass} ${cellToneClass(day)}`;
	}

	function cellTitle(day: TeamStatusPersonDay): string {
		if (day.locationName && day.locationName !== day.label) return `${day.label} · ${day.locationName}`;
		return day.label;
	}

	function segmentBarColor(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.locationColor ?? 'hsl(var(--muted-foreground))';
	}

	function locationIndicatorColor(color: string | undefined): string {
		return color ?? 'hsl(var(--muted-foreground))';
	}

	function calculateEmployeeColumnWidth(employeeRows: TeamStatusPersonRow[]): number {
		const longestNameWidth = Math.max(0, ...employeeRows.map(row => estimateDisplayNameWidth(row.displayName)));
		return clampWidth(longestNameWidth + employeeColumnChromeWidth);
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

	function clampWidth(width: number): number {
		return Math.min(maximumEmployeeColumnWidth, Math.max(minimumEmployeeColumnWidth, width));
	}

	function tooltipTimeLabel(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.durationLabel ? segment.timeLabel : segment.tooltipLabel;
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
					{@const header = dayHeaderParts(date)}
					<div class="p-1 text-center" role="columnheader">
						<button
							type="button"
							class={`flex h-8 w-full flex-col items-center justify-center rounded-sm border text-xs transition ${dayHeaderButtonClass(date)}`}
							data-testid={`team-status-day-${date}`}
							onclick={() => onSelectDate(date)}
						>
							<span class={`font-semibold ${dayHeaderWeekdayClass(date)}`}>{header.weekday}</span>
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
							<div class="min-w-0">
								<div class="truncate text-sm font-medium text-foreground">{row.displayName}</div>
								{#if row.currentLocationName}
									<div class="mt-0.5 flex min-w-0 items-center gap-1 text-[11px] text-foreground">
										<span
											class="size-1.5 shrink-0 rounded-full"
											style:background-color={locationIndicatorColor(row.currentLocationColor)}
										></span>
										<span class="truncate">{row.currentLocationName}</span>
									</div>
								{/if}
							</div>
						</div>
					</div>
					{#each row.days as day, index (day.date)}
						<div class={`flex items-stretch justify-stretch text-center ${index === 0 ? '' : 'border-l'}`} role="cell">
							<Tooltip.Root>
								<Tooltip.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											type="button"
											class={cellButtonClass(day)}
											title={cellTitle(day)}
											data-testid={`team-status-cell-${row.email}-${day.date}`}
											onclick={() => openDayDetail(row, day)}
										>
											<span class="min-w-0 max-w-full truncate text-foreground">{day.label}</span>
											{#if day.segments.length > 0}
												<span class="flex h-1.5 w-[88%] min-w-0 overflow-hidden rounded-full bg-muted" aria-hidden="true">
													{#each day.segments as segment (segment.id)}
														<span
															class="h-full min-w-1"
															style:width={`${segment.sharePercent}%`}
															style:background-color={segmentBarColor(segment)}
														></span>
													{/each}
												</span>
											{:else if day.detailLabel}
												<span class="max-w-full truncate text-[10px] text-foreground/70">{day.detailLabel}</span>
											{/if}
										</button>
									{/snippet}
								</Tooltip.Trigger>
								{#if day.segments.length > 0}
									<Tooltip.Content side="top" sideOffset={6} class="grid w-max max-w-[calc(100vw-2rem)] grid-cols-[0.375rem_max-content_max-content_max-content] gap-x-2 gap-y-1.5 overflow-x-auto">
										{#each day.segments as segment (segment.id)}
											<div class="contents text-left tabular-nums">
												<span
													class="size-1.5 shrink-0 rounded-full"
													style:background-color={segmentBarColor(segment)}
												></span>
												<span class="whitespace-nowrap text-left">{segment.locationName}</span>
												<span class="whitespace-nowrap text-left">{tooltipTimeLabel(segment)}</span>
												<span class="whitespace-nowrap text-left">{segment.durationLabel ?? ''}</span>
											</div>
										{/each}
									</Tooltip.Content>
								{/if}
							</Tooltip.Root>
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
