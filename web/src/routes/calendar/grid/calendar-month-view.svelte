<script lang="ts">
	import { cn } from '$lib/utils';
	import { untrack } from 'svelte';
	import CalendarDayCell from './calendar-day-cell.svelte';
	import CalendarEventChip from './calendar-event-chip.svelte';
	import {
		calendarGridDateFromKey,
		calendarGridDateKey,
		calendarGridDominantMonth,
		calendarGridWeeks,
		isSameCalendarGridDay,
		startOfCalendarGridWeek,
		type CalendarGridWeek
	} from './calendar-grid-dates';
	import { calendarGridWeekLayout, type CalendarGridEvent, type CalendarGridWeekLayout } from './calendar-grid-layout';

	type CalendarMonthViewProps = {
		visibleDate: Date;
		selectedDateKey: string;
		selectedEventID: string;
		events: CalendarGridEvent[];
		localeCode: string;
		moreEventsText: string;
		selectDay: (day: Date) => void;
		openDay: (day: Date) => void;
		addEventOnDay: (day: Date) => void;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		visibleMonthChanged: (month: Date) => void;
	};

	let {
		visibleDate,
		selectedDateKey,
		selectedEventID,
		events,
		localeCode,
		moreEventsText,
		selectDay,
		openDay,
		addEventOnDay,
		openEvent,
		visibleMonthChanged
	}: CalendarMonthViewProps = $props();

	const weeksBeforeVisibleDate = 26;
	const weeksAfterVisibleDate = 26;
	const laneHeightPixels = 22;
	const visibleChipCount = 3;
	const scrollOverlayHideDelayMilliseconds = 700;
	const wheelScrollDamping = 0.3;
	const rowStepThresholdPixels = 24;
	const rowStepCooldownMilliseconds = 240;

	let scrollElement = $state<HTMLElement | null>(null);
	let anchorWeekStartKey = $state('');
	let isScrollOverlayVisible = $state(false);
	let isScrollingToWeek = false;
	let selfReportedMonthKey = '';
	let scrollFrame: number | null = null;
	let scrollOverlayTimer: number | null = null;
	let accumulatedWheelDelta = 0;
	let isSteppingRow = false;

	let windowAnchorDateKey = $state(untrack(() => calendarGridDateKey(visibleDate)));

	const weeks = $derived(calendarGridWeeks(calendarGridDateFromKey(windowAnchorDateKey), weeksBeforeVisibleDate, weeksAfterVisibleDate));
	const weekLayouts = $derived(
		new Map<string, CalendarGridWeekLayout>(weeks.map((week) => [week.startDateKey, calendarGridWeekLayout(week, events)]))
	);
	const weekMonths = $derived(new Map<string, Date>(weeks.map((week) => [week.startDateKey, calendarGridDominantMonth(week)])));
	const weekdayLabels = $derived(
		Array.from({ length: 7 }, (_, weekdayIndex) =>
			new Date(2026, 2, 1 + weekdayIndex).toLocaleDateString(localeCode, { weekday: 'short' })
		)
	);
	const monthLabelFormatter = $derived(new Intl.DateTimeFormat(localeCode, { year: 'numeric', month: 'long' }));
	const timeFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric', minute: '2-digit' }));
	const today = new Date();

	$effect(() => {
		const requestedDateKey = calendarGridDateKey(visibleDate);
		if (requestedDateKey === selfReportedMonthKey) return;
		const visibleWeekStartKey = calendarGridDateKey(startOfCalendarGridWeek(visibleDate));
		if (visibleWeekStartKey === anchorWeekStartKey) return;
		anchorWeekStartKey = visibleWeekStartKey;
		if (!weeks.some((week) => week.startDateKey === visibleWeekStartKey)) windowAnchorDateKey = requestedDateKey;
		scrollWeekIntoView(visibleWeekStartKey);
	});

	$effect(() => {
		return () => {
			if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
			if (scrollOverlayTimer !== null) clearTimeout(scrollOverlayTimer);
		};
	});

	function scrollWeekIntoView(weekStartKey: string, attempt = 0): void {
		requestAnimationFrame(() => {
			const weekElement = scrollElement?.querySelector<HTMLElement>(`[data-week-start="${weekStartKey}"]`);
			if (!weekElement || !scrollElement) {
				if (attempt < 10) scrollWeekIntoView(weekStartKey, attempt + 1);
				return;
			}
			isScrollingToWeek = true;
			scrollElement.scrollTop += weekElement.getBoundingClientRect().top - scrollElement.getBoundingClientRect().top;
			requestAnimationFrame(() => {
				isScrollingToWeek = false;
			});
		});
	}

	function handleWheel(wheelEvent: WheelEvent): void {
		if (!scrollElement || wheelEvent.ctrlKey) return;
		wheelEvent.preventDefault();
		if (isSteppingRow) return;
		accumulatedWheelDelta += wheelDeltaPixels(wheelEvent) * wheelScrollDamping;
		if (Math.abs(accumulatedWheelDelta) < rowStepThresholdPixels) return;
		const direction = accumulatedWheelDelta > 0 ? 1 : -1;
		accumulatedWheelDelta = 0;
		scrollToAdjacentRow(direction);
	}

	function wheelDeltaPixels(wheelEvent: WheelEvent): number {
		const lineHeightPixels = 16;
		if (wheelEvent.deltaMode === 1) return wheelEvent.deltaY * lineHeightPixels;
		if (wheelEvent.deltaMode === 2) return wheelEvent.deltaY * (scrollElement?.clientHeight ?? 0);
		return wheelEvent.deltaY;
	}

	function scrollToAdjacentRow(direction: 1 | -1): void {
		if (!scrollElement) return;
		const viewportTop = scrollElement.getBoundingClientRect().top;
		const rowElements = [...scrollElement.querySelectorAll<HTMLElement>('[data-week-start]')];
		const currentRowIndex = rowElements.findIndex((rowElement) => rowElement.getBoundingClientRect().bottom > viewportTop + 2);
		const targetRow = rowElements[Math.min(rowElements.length - 1, Math.max(0, currentRowIndex + direction))];
		if (!targetRow) return;
		isSteppingRow = true;
		scrollElement.scrollTo({
			top: scrollElement.scrollTop + targetRow.getBoundingClientRect().top - viewportTop,
			behavior: 'smooth'
		});
		window.setTimeout(() => {
			isSteppingRow = false;
		}, rowStepCooldownMilliseconds);
	}

	function handleScroll(): void {
		if (isScrollingToWeek || scrollFrame !== null) return;
		scrollFrame = requestAnimationFrame(() => {
			scrollFrame = null;
			updateVisibleWeek();
		});
	}

	function updateVisibleWeek(): void {
		if (!scrollElement) return;
		const scrollTop = scrollElement.getBoundingClientRect().top;
		const weekElements = scrollElement.querySelectorAll<HTMLElement>('[data-week-start]');
		let topWeekStartKey = '';
		for (const weekElement of weekElements) {
			if (weekElement.getBoundingClientRect().bottom <= scrollTop + 8) continue;
			topWeekStartKey = weekElement.dataset.weekStart ?? '';
			break;
		}
		if (!topWeekStartKey || topWeekStartKey === anchorWeekStartKey) return;
		anchorWeekStartKey = topWeekStartKey;
		const month = weekMonths.get(topWeekStartKey);
		if (!month) return;
		showScrollOverlay();
		selfReportedMonthKey = calendarGridDateKey(month);
		visibleMonthChanged(month);
	}

	function showScrollOverlay(): void {
		isScrollOverlayVisible = true;
		if (scrollOverlayTimer !== null) clearTimeout(scrollOverlayTimer);
		scrollOverlayTimer = window.setTimeout(() => {
			scrollOverlayTimer = null;
			isScrollOverlayVisible = false;
		}, scrollOverlayHideDelayMilliseconds);
	}

	function timedEventsForDay(layout: CalendarGridWeekLayout, dayIndex: number) {
		const entries = layout.timedEntries.filter((entry) => entry.dayIndex === dayIndex);
		const availableChipCount = Math.max(1, visibleChipCount - layout.laneCount);
		return {
			visible: entries.slice(0, availableChipCount),
			hiddenCount: Math.max(0, entries.length - availableChipCount)
		};
	}

	function isOutsideVisibleMonth(day: Date, week: CalendarGridWeek): boolean {
		return day.getMonth() !== (weekMonths.get(week.startDateKey)?.getMonth() ?? day.getMonth());
	}

	function monthStartDayInWeek(week: CalendarGridWeek): Date | undefined {
		return week.days.find((day) => day.getDate() === 1);
	}
</script>

<div class="relative flex min-h-0 flex-1 flex-col">
	<div class="border-border/70 text-muted-foreground grid grid-cols-7 border-b text-xs font-medium">
		{#each weekdayLabels as weekdayLabel, weekdayIndex (weekdayLabel)}
			<div class={cn('px-2 py-1.5', weekdayIndex === 0 && 'text-destructive', weekdayIndex === 6 && 'text-primary')}>
				{weekdayLabel}
			</div>
		{/each}
	</div>
	<div bind:this={scrollElement} onscroll={handleScroll} onwheel={handleWheel} class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
		{#each weeks as week (week.startDateKey)}
			{@const layout = weekLayouts.get(week.startDateKey) ?? { spans: [], timedEntries: [], laneCount: 0 }}
			{@const monthStartDay = monthStartDayInWeek(week)}
			<div data-week-start={week.startDateKey} class="relative grid grid-cols-7">
				{#if monthStartDay}
					<div
						aria-hidden="true"
						class={cn(
							'text-foreground/75 pointer-events-none absolute top-1.5 left-6 z-10 text-[22px] leading-none font-extrabold tabular-nums transition-opacity duration-200',
							isScrollOverlayVisible ? 'opacity-100' : 'opacity-0'
						)}
						style="text-shadow: 0 4px 16px var(--color-background)"
					>
						{monthLabelFormatter.format(monthStartDay)}
					</div>
				{/if}
				{#each week.days as day, dayIndex (day.getTime())}
					{@const timed = timedEventsForDay(layout, dayIndex)}
					<CalendarDayCell
						{day}
						dayLabel={String(day.getDate())}
						isToday={isSameCalendarGridDay(day, today)}
						isOutsideMonth={isOutsideVisibleMonth(day, week)}
						isSelected={selectedDateKey === calendarGridDateKey(day)}
						addEventOnDay={(selectedDay) => addEventOnDay(selectedDay)}
						selectDay={(selectedDay) => selectDay(selectedDay)}
					>
						<div style={`height: ${layout.laneCount * laneHeightPixels}px`}></div>
						{#each timed.visible as entry (entry.event.id)}
							<CalendarEventChip
								event={entry.event}
								timeLabel={entry.event.isAllDay ? '' : timeFormatter.format(entry.event.start)}
								isSelected={selectedEventID === entry.event.id}
								{openEvent}
							/>
						{/each}
						{#if timed.hiddenCount > 0}
							<button
								type="button"
								class="text-muted-foreground hover:text-foreground px-1.5 text-left text-xs"
								onclick={() => openDay(day)}
							>
								{moreEventsText.replace('{count}', String(timed.hiddenCount))}
							</button>
						{/if}
					</CalendarDayCell>
				{/each}
				<div class="pointer-events-none absolute inset-x-0 top-8 grid grid-cols-7">
					{#each layout.spans as span (span.event.id)}
						<div
							class="pointer-events-auto px-1"
							style={`grid-column: ${span.startColumn + 1} / span ${span.columnCount}; grid-row: 1; margin-top: ${span.lane * laneHeightPixels}px`}
						>
							<CalendarEventChip
								event={span.event}
								variant="span"
								isSelected={selectedEventID === span.event.id}
								continuesBefore={span.continuesBefore}
								continuesAfter={span.continuesAfter}
								{openEvent}
							/>
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
</div>
