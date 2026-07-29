<script lang="ts">
	import { cn } from '$lib/utils';
	import { tick, untrack } from 'svelte';
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
	import { createCalendarScrollSnap } from './calendar-grid-scroll-snap.svelte';

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
		addEventOnRange: (startDateKey: string, endDateKey: string) => void;
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
		addEventOnRange,
		openEvent,
		visibleMonthChanged
	}: CalendarMonthViewProps = $props();

	const weeksBeforeVisibleDate = 26;
	const weeksAfterVisibleDate = 26;
	const laneHeightPixels = 22;
	const visibleChipCount = 3;
	const scrollOverlayHideDelayMilliseconds = 700;
	const wheelScrollDamping = 0.7;
	const rowSnapAnimationMilliseconds = 220;
	const windowExtendWeeks = 26;
	const windowExtendMarginPixels = 1200;
	const longPressMilliseconds = 450;
	const rangeDragThresholdPixels = 6;

	let scrollElement = $state<HTMLElement | null>(null);
	let anchorWeekStartKey = $state('');
	let isScrollOverlayVisible = $state(false);
	let isScrollingToWeek = false;
	let selfReportedMonthKey = '';
	let scrollFrame: number | null = null;
	let scrollOverlayTimer: number | null = null;
	let isExtendingWindow = false;
	let rangeStartDateKey = $state('');
	let rangeEndDateKey = $state('');
	let isRangeDragging = $state(false);
	let rangeStartPoint: { clientX: number; clientY: number } | null = null;
	let longPressTimer: number | null = null;

	let windowAnchorDateKey = $state(untrack(() => calendarGridDateKey(visibleDate)));
	let weeksBeforeAnchor = $state(weeksBeforeVisibleDate);
	let weeksAfterAnchor = $state(weeksAfterVisibleDate);

	const weeks = $derived(calendarGridWeeks(calendarGridDateFromKey(windowAnchorDateKey), weeksBeforeAnchor, weeksAfterAnchor));
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
	const activeMonth = $derived(weekMonths.get(anchorWeekStartKey) ?? new Date(visibleDate.getFullYear(), visibleDate.getMonth(), 1));
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

	const scrollSnap = createCalendarScrollSnap({
		getScrollElement: () => scrollElement,
		getSnapOffsets: weekRowScrollOffsets,
		damping: wheelScrollDamping,
		animationMilliseconds: rowSnapAnimationMilliseconds
	});

	function weekRowScrollOffsets(): number[] {
		if (!scrollElement) return [];
		const viewportTop = scrollElement.getBoundingClientRect().top;
		const currentScrollTop = scrollElement.scrollTop;
		return [...scrollElement.querySelectorAll<HTMLElement>('[data-week-start]')].map(
			(rowElement) => currentScrollTop + rowElement.getBoundingClientRect().top - viewportTop
		);
	}

	$effect(() => {
		return () => {
			if (scrollFrame !== null) cancelAnimationFrame(scrollFrame);
			if (scrollOverlayTimer !== null) clearTimeout(scrollOverlayTimer);
			if (longPressTimer !== null) clearTimeout(longPressTimer);
			scrollSnap.destroy();
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

	function handleScroll(): void {
		if (isScrollingToWeek || scrollFrame !== null) return;
		scrollFrame = requestAnimationFrame(() => {
			scrollFrame = null;
			updateVisibleWeek();
			void extendWindowNearEdges();
		});
	}

	async function extendWindowNearEdges(): Promise<void> {
		if (!scrollElement || isExtendingWindow) return;
		const distanceToTop = scrollElement.scrollTop;
		const distanceToBottom = scrollElement.scrollHeight - scrollElement.scrollTop - scrollElement.clientHeight;
		if (distanceToTop > windowExtendMarginPixels && distanceToBottom > windowExtendMarginPixels) return;
		isExtendingWindow = true;
		const isNearTop = distanceToTop <= windowExtendMarginPixels;
		const scrollHeightBefore = scrollElement.scrollHeight;
		if (isNearTop) weeksBeforeAnchor += windowExtendWeeks;
		else weeksAfterAnchor += windowExtendWeeks;
		await tick();
		if (scrollElement && isNearTop) scrollElement.scrollTop += scrollElement.scrollHeight - scrollHeightBefore;
		isExtendingWindow = false;
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

	function isOutsideVisibleMonth(day: Date): boolean {
		return day.getMonth() !== activeMonth.getMonth() || day.getFullYear() !== activeMonth.getFullYear();
	}

	function monthStartDayInWeek(week: CalendarGridWeek): Date | undefined {
		return week.days.find((day) => day.getDate() === 1);
	}

	function dateKeyFromPoint(clientX: number, clientY: number): string {
		const element = scrollElement?.ownerDocument.elementFromPoint(clientX, clientY);
		return element?.closest<HTMLElement>('[data-calendar-date]')?.dataset.calendarDate ?? '';
	}

	function handlePointerDown(pointerEvent: PointerEvent): void {
		if (pointerEvent.button !== 0 || !(pointerEvent.target instanceof Element)) return;
		if (pointerEvent.target.closest('button')) return;
		const dateKey = dateKeyFromPoint(pointerEvent.clientX, pointerEvent.clientY);
		if (!dateKey) return;
		rangeStartDateKey = dateKey;
		rangeEndDateKey = dateKey;
		rangeStartPoint = { clientX: pointerEvent.clientX, clientY: pointerEvent.clientY };
		isRangeDragging = false;
		longPressTimer = window.setTimeout(() => {
			longPressTimer = null;
			if (isRangeDragging || !rangeStartDateKey) return;
			const pressedDateKey = rangeStartDateKey;
			clearRangeSelection();
			addEventOnDay(calendarGridDateFromKey(pressedDateKey));
		}, longPressMilliseconds);
	}

	function handlePointerMove(pointerEvent: PointerEvent): void {
		if (!rangeStartDateKey || !rangeStartPoint) return;
		const movedDistance =
			Math.abs(pointerEvent.clientX - rangeStartPoint.clientX) + Math.abs(pointerEvent.clientY - rangeStartPoint.clientY);
		if (!isRangeDragging && movedDistance < rangeDragThresholdPixels) return;
		isRangeDragging = true;
		cancelLongPress();
		const dateKey = dateKeyFromPoint(pointerEvent.clientX, pointerEvent.clientY);
		if (dateKey) rangeEndDateKey = dateKey;
	}

	function handlePointerUp(): void {
		cancelLongPress();
		if (!isRangeDragging || !rangeStartDateKey || !rangeEndDateKey) {
			clearRangeSelection();
			return;
		}
		const [startDateKey, endDateKey] = [rangeStartDateKey, rangeEndDateKey].sort();
		clearRangeSelection();
		addEventOnRange(startDateKey, endDateKey);
	}

	function cancelLongPress(): void {
		if (longPressTimer === null) return;
		clearTimeout(longPressTimer);
		longPressTimer = null;
	}

	function clearRangeSelection(): void {
		cancelLongPress();
		rangeStartDateKey = '';
		rangeEndDateKey = '';
		rangeStartPoint = null;
		isRangeDragging = false;
	}

	function isDateKeyInSelectedRange(dateKey: string): boolean {
		if (!isRangeDragging || !rangeStartDateKey || !rangeEndDateKey) return false;
		const [startDateKey, endDateKey] = [rangeStartDateKey, rangeEndDateKey].sort();
		return dateKey >= startDateKey && dateKey <= endDateKey;
	}
</script>

<div class="relative flex min-h-0 flex-1 flex-col">
	<div class="border-border/50 text-muted-foreground grid grid-cols-7 border-b text-xs font-medium">
		{#each weekdayLabels as weekdayLabel, weekdayIndex (weekdayLabel)}
			<div class={cn('px-2 py-1.5', weekdayIndex === 0 && 'text-destructive', weekdayIndex === 6 && 'text-primary')}>
				{weekdayLabel}
			</div>
		{/each}
	</div>
	<div
		bind:this={scrollElement}
		role="grid"
		tabindex="-1"
		aria-label={monthLabelFormatter.format(visibleDate)}
		onscroll={handleScroll}
		onscrollend={scrollSnap.handleScrollEnd}
		onwheel={scrollSnap.handleWheel}
		ontouchend={scrollSnap.handleGestureEnd}
		onpointerdown={handlePointerDown}
		onpointermove={handlePointerMove}
		onpointerup={handlePointerUp}
		onpointercancel={clearRangeSelection}
		onpointerleave={clearRangeSelection} class="no-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-contain">
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
						isOutsideMonth={isOutsideVisibleMonth(day)}
						isSelected={selectedDateKey === calendarGridDateKey(day)}
						isInSelectedRange={isDateKeyInSelectedRange(calendarGridDateKey(day))}
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
