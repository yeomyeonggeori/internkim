<script lang="ts">
	import { cn } from '$lib/utils';
	import CalendarDayCell from './calendar-day-cell.svelte';
	import CalendarEventChip from './calendar-event-chip.svelte';
	import {
		addCalendarGridDays,
		calendarGridDateKey,
		calendarGridDominantMonth,
		calendarGridWeeks,
		isSameCalendarGridDay,
		startOfCalendarGridWeek,
		type CalendarGridWeek
	} from './calendar-grid-dates';
	import { calendarGridWeekLayout, type CalendarGridEvent } from './calendar-grid-layout';

	type CalendarMonthViewProps = {
		visibleDate: Date;
		selectedDateKey: string;
		selectedEventID: string;
		events: CalendarGridEvent[];
		localeCode: string;
		text: {
			addEventOnDay: string;
			openDay: string;
			editEvent: string;
			duplicateEvent: string;
			deleteEvent: string;
			moreEvents: string;
		};
		selectDay: (day: Date) => void;
		openDay: (day: Date) => void;
		addEventOnDay: (day: Date) => void;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		duplicateEvent: (event: CalendarGridEvent) => void;
		deleteEvent: (event: CalendarGridEvent) => void;
		visibleMonthChanged: (month: Date) => void;
	};

	let {
		visibleDate,
		selectedDateKey,
		selectedEventID,
		events,
		localeCode,
		text,
		selectDay,
		openDay,
		addEventOnDay,
		openEvent,
		duplicateEvent,
		deleteEvent,
		visibleMonthChanged
	}: CalendarMonthViewProps = $props();

	const weeksBeforeVisibleDate = 26;
	const weeksAfterVisibleDate = 26;
	const laneHeightPixels = 22;
	const visibleChipCount = 3;

	let scrollElement = $state<HTMLElement | null>(null);
	let anchorWeekStartKey = $state('');
	let isScrollingToWeek = false;

	const weeks = $derived(calendarGridWeeks(visibleDate, weeksBeforeVisibleDate, weeksAfterVisibleDate));
	const weekdayLabels = $derived(
		Array.from({ length: 7 }, (_, weekdayIndex) =>
			new Date(2026, 2, 1 + weekdayIndex).toLocaleDateString(localeCode, { weekday: 'short' })
		)
	);
	const timeFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric', minute: '2-digit' }));

	$effect(() => {
		const visibleWeekStartKey = calendarGridDateKey(startOfCalendarGridWeek(visibleDate));
		if (visibleWeekStartKey === anchorWeekStartKey) return;
		anchorWeekStartKey = visibleWeekStartKey;
		scrollWeekIntoView(visibleWeekStartKey);
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

	function handleScroll(): void {
		if (!scrollElement || isScrollingToWeek) return;
		const weekElements = Array.from(scrollElement.querySelectorAll<HTMLElement>('[data-week-start]'));
		const topWeekElement = weekElements.find((element) => element.offsetTop + element.offsetHeight > scrollElement!.scrollTop + 8);
		const weekStartKey = topWeekElement?.dataset.weekStart;
		if (!weekStartKey || weekStartKey === anchorWeekStartKey) return;
		anchorWeekStartKey = weekStartKey;
		const week = weeks.find((candidate) => candidate.startDateKey === weekStartKey);
		if (week) visibleMonthChanged(calendarGridDominantMonth(week));
	}

	function weekEvents(week: CalendarGridWeek) {
		return calendarGridWeekLayout(week, events);
	}

	function timedEventsForDay(week: CalendarGridWeek, dayIndex: number, laneCount: number) {
		const layout = weekEvents(week);
		const entries = layout.timedEntries.filter((entry) => entry.dayIndex === dayIndex);
		const availableChipCount = Math.max(1, visibleChipCount - laneCount);
		return {
			visible: entries.slice(0, availableChipCount),
			hiddenCount: Math.max(0, entries.length - availableChipCount)
		};
	}

	function isOutsideVisibleMonth(day: Date, week: CalendarGridWeek): boolean {
		return day.getMonth() !== calendarGridDominantMonth(week).getMonth();
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="border-border/70 text-muted-foreground grid grid-cols-7 border-b text-xs font-medium">
		{#each weekdayLabels as weekdayLabel, weekdayIndex (weekdayLabel)}
			<div class={cn('px-2 py-1.5', weekdayIndex === 0 && 'text-destructive', weekdayIndex === 6 && 'text-primary')}>
				{weekdayLabel}
			</div>
		{/each}
	</div>
	<div bind:this={scrollElement} onscroll={handleScroll} class="min-h-0 flex-1 overflow-y-auto">
		{#each weeks as week (week.startDateKey)}
			{@const layout = weekEvents(week)}
			<div data-week-start={week.startDateKey} class="relative grid grid-cols-7">
				{#each week.days as day, dayIndex (day.getTime())}
					{@const timed = timedEventsForDay(week, dayIndex, layout.laneCount)}
					<CalendarDayCell
						{day}
						dayLabel={String(day.getDate())}
						isToday={isSameCalendarGridDay(day, new Date())}
						isOutsideMonth={isOutsideVisibleMonth(day, week)}
						isSelected={selectedDateKey === calendarGridDateKey(day)}
						text={{ addEventOnDay: text.addEventOnDay }}
						addEventOnDay={(selectedDay) => addEventOnDay(selectedDay)}
						selectDay={(selectedDay) => selectDay(selectedDay)}
					>
						<div style={`height: ${layout.laneCount * laneHeightPixels}px`}></div>
						{#each timed.visible as entry (entry.event.id)}
							<CalendarEventChip
								event={entry.event}
								timeLabel={entry.event.isAllDay ? '' : timeFormatter.format(entry.event.start)}
								isSelected={selectedEventID === entry.event.id}
								text={{ editEvent: text.editEvent, duplicateEvent: text.duplicateEvent, deleteEvent: text.deleteEvent }}
								{openEvent}
								{duplicateEvent}
								{deleteEvent}
							/>
						{/each}
						{#if timed.hiddenCount > 0}
							<button
								type="button"
								class="text-muted-foreground hover:text-foreground px-1.5 text-left text-xs"
								onclick={() => openDay(day)}
							>
								{text.moreEvents.replace('{count}', String(timed.hiddenCount))}
							</button>
						{/if}
					</CalendarDayCell>
				{/each}
				<div class="pointer-events-none absolute inset-x-0 top-8 grid grid-cols-7 gap-x-0 px-0">
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
								text={{ editEvent: text.editEvent, duplicateEvent: text.duplicateEvent, deleteEvent: text.deleteEvent }}
								{openEvent}
								{duplicateEvent}
								{deleteEvent}
							/>
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
</div>
