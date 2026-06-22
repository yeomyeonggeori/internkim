<script lang="ts">
	import { isWeekend } from '../shared/attendance-date';
	import { absenceDisplayClass } from '../shared/color-tokens';
	import {
		teamCalendarVisibleLaneCount,
		type TeamCalendarAbsence,
		type TeamCalendarDayCell
	} from './team-month-calendar-model';

	type Props = {
		cell: TeamCalendarDayCell;
		absences: TeamCalendarAbsence[];
		isSelected: boolean;
		isToday: boolean;
		countLabel: string;
		moreLabel: string;
		onSelect: (date: string) => void;
	};

	let { cell, absences, isSelected, isToday, countLabel, moreLabel, onSelect }: Props = $props();
	let rootElement: HTMLDivElement | undefined = $state();
	let isPopoverOpen = $state(false);

	const visibleAbsenceSlots = $derived(
		Array.from({ length: teamCalendarVisibleLaneCount }, (_, lane) =>
			absences.find((absence) => absence.isVisible && absence.lane === lane)
		)
	);
	const hiddenAbsenceCount = $derived(absences.filter((absence) => !absence.isVisible).length);
	const dayNumber = $derived(Number(cell.date.slice(-2)));
	const isWeekendDay = $derived(isWeekend(cell.date));

	$effect(() => {
		if (hiddenAbsenceCount === 0) isPopoverOpen = false;
	});

	function absenceClass(absence: TeamCalendarAbsence): string {
		return absenceDisplayClass(absence.tone, true);
	}

	function absenceShapeClass(absence: TeamCalendarAbsence): string {
		const widthClass = absence.starts && absence.ends
			? 'w-full'
			: absence.starts
				? 'w-[calc(100%+7px)]'
				: absence.ends
					? 'w-[calc(100%+7px)] -translate-x-[7px]'
					: 'w-[calc(100%+14px)] -translate-x-[7px]';
		const startClass = absence.starts ? 'rounded-l-sm border-l-2 pl-1.5' : 'rounded-l-none border-l-0 pl-1.5';
		const endClass = absence.ends ? 'rounded-r-sm pr-1.5' : 'rounded-r-none pr-1.5';
		return `${widthClass} ${startClass} ${endClass}`;
	}

	function popoverClass(): string {
		const alignment = new Date(`${cell.date}T00:00:00Z`).getUTCDay() >= 5 ? 'right-1' : 'left-1';
		return `${alignment} w-48 max-w-[calc(100vw-2rem)]`;
	}

	function cellClass(): string {
		if (!cell.inCurrentMonth) return 'bg-muted/20 text-muted-foreground opacity-40';
		const weekend = isWeekendDay ? 'bg-muted/40 text-destructive/70' : 'bg-background hover:bg-muted/40';
		return `border-border/50 ${weekend}`;
	}

	function selectCurrentDate() {
		if (!cell.inCurrentMonth) return;
		isPopoverOpen = false;
		onSelect(cell.date);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		selectCurrentDate();
	}

	function togglePopover(event: MouseEvent) {
		event.stopPropagation();
		isPopoverOpen = !isPopoverOpen;
	}

	function handleDocumentPointerdown(event: PointerEvent) {
		if (!isPopoverOpen) return;
		if (event.target instanceof Node && rootElement?.contains(event.target)) return;
		isPopoverOpen = false;
	}
</script>

<svelte:document onpointerdown={handleDocumentPointerdown} />

<div
	bind:this={rootElement}
	role="button"
	tabindex={cell.inCurrentMonth ? 0 : -1}
	aria-disabled={!cell.inCurrentMonth}
	data-testid={`team-calendar-day-${cell.date}`}
	class={`relative flex h-28 min-w-0 flex-col overflow-visible border p-1 text-left transition ${cellClass()}`}
	onclick={selectCurrentDate}
	onkeydown={handleKeydown}
>
	<div class="flex items-start justify-between gap-1">
		<span class={`flex size-5 items-center justify-center text-xs font-medium tabular-nums ${isToday ? 'rounded-full bg-primary text-primary-foreground' : ''}`}>
			{dayNumber}
		</span>
	</div>
	<div class="mt-px flex min-h-0 flex-1 flex-col gap-0.5 overflow-visible">
		{#each visibleAbsenceSlots as absence, lane (lane)}
			{#if absence}
				<span
					class={`relative z-20 block h-5 truncate text-[10px] font-medium leading-5 ${absenceShapeClass(absence)} ${absenceClass(absence)}`}
					title={absence.label}
				>
					{absence.starts ? absence.label : ''}
				</span>
			{:else}
				<span class="block h-5"></span>
			{/if}
		{/each}
	</div>
	<div class="absolute bottom-1 left-1 right-1 flex items-center justify-between gap-1">
		{#if hiddenAbsenceCount > 0}
			<button
				type="button"
				class="rounded px-1 text-[10px] font-medium leading-4 text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
				aria-expanded={isPopoverOpen}
				onclick={togglePopover}
			>
				{moreLabel}
			</button>
		{:else}
			<span></span>
		{/if}
		{#if cell.presentCount > 0 && cell.totalPeople > 0}
			<span class="text-[10px] font-semibold leading-4 text-muted-foreground tabular-nums">
				{countLabel}
			</span>
		{/if}
	</div>
	{#if isSelected}
		<div class="pointer-events-none absolute inset-0 z-30 border border-foreground shadow-xs ring-1 ring-foreground/20"></div>
	{/if}
	{#if isPopoverOpen}
		<div class={`absolute bottom-7 z-40 rounded-md border bg-popover p-1 text-popover-foreground shadow-md ${popoverClass()}`}>
			{#each absences as absence, index (`${absence.key}:${index}`)}
				<div
					class={`mb-0.5 grid h-5 min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 px-1 text-[10px] font-medium leading-5 last:mb-0 ${absenceClass(absence)}`}
					title={`${absence.label} ${absence.periodLabel}`}
				>
					<span class="min-w-0 truncate">{absence.label}</span>
					<span class="shrink-0 text-[9px] opacity-75">{absence.periodLabel}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>
