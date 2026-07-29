<script lang="ts">
	import { Calendar as MiniCalendar } from '$lib/components/ui/calendar';
	import { cn } from '$lib/utils';
	import { CalendarDate, type DateValue } from '@internationalized/date';
	import CalendarEventChip from './calendar-event-chip.svelte';
	import {
		addCalendarGridDays,
		calendarGridDateAtMinutes,
		calendarGridDateFromKey,
		calendarGridDateKey,
		calendarGridMinutesFromMidnight,
		isSameCalendarGridDay,
		startOfCalendarGridDay,
		startOfCalendarGridWeek
	} from './calendar-grid-dates';
	import { calendarGridTimedBlocks, type CalendarGridEvent } from './calendar-grid-layout';
	import { createCalendarScrollSnap } from './calendar-grid-scroll-snap.svelte';

	type CalendarTimeViewProps = {
		visibleDate: Date;
		dayCount: number;
		selectedEventID: string;
		events: CalendarGridEvent[];
		localeCode: string;
		selectDay: (day: Date) => void;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		addEventOnTimeRange: (start: Date, end: Date) => void;
	};

	let {
		visibleDate,
		dayCount,
		selectedEventID,
		events,
		localeCode,
		selectDay,
		openEvent,
		addEventOnTimeRange
	}: CalendarTimeViewProps = $props();

	const hourHeightPixels = 48;
	const minutesPerDay = 24 * 60;
	const dragSnapMinutes = 15;
	const longPressMilliseconds = 450;
	const defaultDurationMinutes = 60;
	const flickMilliseconds = 260;
	const flickTravelPixels = 40;

	let gridElement = $state<HTMLElement | null>(null);
	let draftStartMinutes = $state<number | null>(null);
	let draftEndMinutes = $state<number | null>(null);
	let draftDay = $state<Date | null>(null);
	let longPressTimer: number | null = null;
	let weekWindowStartKey = $state('');
	let selfReportedLeadingDayKey = '';
	let columnsElement = $state<HTMLElement | null>(null);
	let headerStripElement = $state<HTMLElement | null>(null);
	let isRecenteringColumns = false;

	const weekWindowStart = $derived(
		weekWindowStartKey ? calendarGridDateFromKey(weekWindowStartKey) : startOfCalendarGridWeek(visibleDate)
	);
	const stripDays = $derived(Array.from({ length: 21 }, (_, dayOffset) => addCalendarGridDays(weekWindowStart, dayOffset - 7)));
	const days = $derived(
		dayCount === 1
			? [startOfCalendarGridDay(visibleDate)]
			: Array.from({ length: dayCount }, (_, dayOffset) => addCalendarGridDays(weekWindowStart, dayOffset))
	);
	const canSwipeWeeks = $derived(dayCount === 7);
	const allDayEvents = $derived(
		days.map((day) => ({
			day,
			events: events.filter(
				(event) => event.isAllDay && event.end > startOfCalendarGridDay(day) && event.start < addCalendarGridDays(day, 1)
			)
		}))
	);
	const weekdayFormatter = $derived(new Intl.DateTimeFormat(localeCode, { weekday: 'short' }));
	const hourFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric' }));
	const timeFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric', minute: '2-digit' }));
	const today = new Date();
	const nowMinutes = $derived(calendarGridMinutesFromMidnight(today));
	const miniCalendarValue = $derived(
		new CalendarDate(visibleDate.getFullYear(), visibleDate.getMonth() + 1, visibleDate.getDate()) as DateValue
	);

	function selectMiniCalendarDate(dateValue: DateValue | undefined): void {
		if (!dateValue) return;
		selectDay(new Date(dateValue.year, dateValue.month - 1, dateValue.day, 12, 0, 0, 0));
	}

	const scrollSnap = createCalendarScrollSnap({
		getScrollElement: () => gridElement,
		getSnapOffsets: hourScrollOffsets
	});












	function hourScrollOffsets(): number[] {
		return Array.from({ length: 24 }, (_, hour) => hour * hourHeightPixels);
	}

	$effect(() => {
		if (!canSwipeWeeks) return;
		const requestedDayKey = calendarGridDateKey(visibleDate);
		if (requestedDayKey === selfReportedLeadingDayKey) return;
		const weekStartKey = calendarGridDateKey(startOfCalendarGridWeek(visibleDate));
		if (weekWindowStartKey === weekStartKey) return;
		weekWindowStartKey = weekStartKey;
		recenterColumns();
	});

	const columnSnap = createCalendarScrollSnap({
		getScrollElement: () => columnsElement,
		getSnapOffsets: columnSnapOffsets,
		axis: 'horizontal',
		damping: 1,
		resolveSnapOffset: resolveColumnSnapOffset,
		onSnapSettled: settleLeadingColumn
	});

	function resolveColumnSnapOffset({
		currentOffset,
		gestureStartOffset,
		offsets,
		gestureMilliseconds
	}: {
		currentOffset: number;
		gestureStartOffset: number;
		offsets: number[];
		gestureMilliseconds: number;
	}): number {
		const columnWidth = columnWidthPixels();
		if (columnWidth === 0 || offsets.length === 0) return currentOffset;
		const travel = currentOffset - gestureStartOffset;
		const isFlick = gestureMilliseconds < flickMilliseconds && Math.abs(travel) >= flickTravelPixels;
		const startColumnIndex = Math.round(gestureStartOffset / columnWidth);
		const targetColumnIndex = isFlick
			? startColumnIndex + (travel > 0 ? 7 : -7)
			: Math.round(currentOffset / columnWidth);
		const boundedIndex = Math.min(offsets.length - 1, Math.max(0, targetColumnIndex));
		return boundedIndex * columnWidth;
	}

	function columnWidthPixels(): number {
		return (columnsElement?.clientWidth ?? 0) / 7;
	}

	function columnSnapOffsets(): number[] {
		const columnWidth = columnWidthPixels();
		if (columnWidth === 0) return [];
		return Array.from({ length: 15 }, (_, columnIndex) => columnIndex * columnWidth);
	}

	function handleGridWheel(wheelEvent: WheelEvent): void {
		if (!canSwipeWeeks || Math.abs(wheelEvent.deltaX) <= Math.abs(wheelEvent.deltaY)) {
			scrollSnap.handleWheel(wheelEvent);
			return;
		}
		columnSnap.handleWheel(wheelEvent);
	}

	function settleLeadingColumn(offset: number): void {
		if (!columnsElement || isRecenteringColumns) return;
		const columnWidth = columnWidthPixels();
		if (columnWidth === 0) return;
		const columnIndex = Math.round(offset / columnWidth);
		if (columnIndex === 7) return;
		const leadingDay = stripDays[columnIndex];
		if (!leadingDay) return;
		selfReportedLeadingDayKey = calendarGridDateKey(leadingDay);
		weekWindowStartKey = calendarGridDateKey(leadingDay);
		recenterColumns();
		selectDay(leadingDay);
	}

	function syncHeaderStripScroll(): void {
		if (!columnsElement || !headerStripElement) return;
		headerStripElement.scrollLeft = columnsElement.scrollLeft;
	}

	function recenterColumns(): void {
		isRecenteringColumns = true;
		requestAnimationFrame(() => {
			if (columnsElement) columnsElement.scrollLeft = columnWidthPixels() * 7;
			syncHeaderStripScroll();
			requestAnimationFrame(() => {
				isRecenteringColumns = false;
			});
		});
	}

	$effect(() => {
		if (!canSwipeWeeks || !columnsElement) return;
		weekWindowStartKey;
		recenterColumns();
	});


	$effect(() => {
		return () => {
			if (longPressTimer !== null) clearTimeout(longPressTimer);
			scrollSnap.destroy();
			columnSnap.destroy();
		};
	});

	function minutesFromPoint(dayColumn: HTMLElement, clientY: number): number {
		const columnRectangle = dayColumn.getBoundingClientRect();
		const offsetMinutes = ((clientY - columnRectangle.top) / hourHeightPixels) * 60;
		const snappedMinutes = Math.round(offsetMinutes / dragSnapMinutes) * dragSnapMinutes;
		return Math.max(0, Math.min(minutesPerDay, snappedMinutes));
	}

	function handleColumnPointerDown(pointerEvent: PointerEvent, day: Date): void {
		if (pointerEvent.button !== 0 || !(pointerEvent.currentTarget instanceof HTMLElement)) return;
		if (pointerEvent.target instanceof Element && pointerEvent.target.closest('button')) return;
		const startMinutes = minutesFromPoint(pointerEvent.currentTarget, pointerEvent.clientY);
		draftDay = day;
		draftStartMinutes = startMinutes;
		draftEndMinutes = startMinutes;
		pointerEvent.currentTarget.setPointerCapture(pointerEvent.pointerId);
		longPressTimer = window.setTimeout(() => {
			longPressTimer = null;
			if (draftStartMinutes === null || draftEndMinutes !== draftStartMinutes) return;
			commitDraft(draftStartMinutes + defaultDurationMinutes);
		}, longPressMilliseconds);
	}

	function handleColumnPointerMove(pointerEvent: PointerEvent): void {
		if (draftStartMinutes === null || !(pointerEvent.currentTarget instanceof HTMLElement)) return;
		draftEndMinutes = minutesFromPoint(pointerEvent.currentTarget, pointerEvent.clientY);
	}

	function handleColumnPointerUp(): void {
		if (draftStartMinutes === null || draftEndMinutes === null) return;
		commitDraft(draftEndMinutes);
	}

	function commitDraft(endMinutes: number): void {
		cancelLongPress();
		const day = draftDay;
		const startMinutes = draftStartMinutes;
		clearDraft();
		if (!day || startMinutes === null) return;
		const [firstMinutes, lastMinutes] = [startMinutes, endMinutes].sort((first, second) => first - second);
		const rangeEndMinutes = lastMinutes - firstMinutes < dragSnapMinutes ? firstMinutes + defaultDurationMinutes : lastMinutes;
		addEventOnTimeRange(calendarGridDateAtMinutes(day, firstMinutes), calendarGridDateAtMinutes(day, rangeEndMinutes));
	}

	function cancelLongPress(): void {
		if (longPressTimer === null) return;
		clearTimeout(longPressTimer);
		longPressTimer = null;
	}

	function clearDraft(): void {
		cancelLongPress();
		draftDay = null;
		draftStartMinutes = null;
		draftEndMinutes = null;
	}

	function draftRangeForDay(day: Date): { topPixels: number; heightPixels: number } | null {
		if (!draftDay || draftStartMinutes === null || draftEndMinutes === null) return null;
		if (!isSameCalendarGridDay(draftDay, day)) return null;
		const [firstMinutes, lastMinutes] = [draftStartMinutes, draftEndMinutes].sort((first, second) => first - second);
		return {
			topPixels: (firstMinutes / 60) * hourHeightPixels,
			heightPixels: Math.max(hourHeightPixels / 4, ((lastMinutes - firstMinutes) / 60) * hourHeightPixels)
		};
	}
</script>

{#snippet dayColumn(day: Date)}
	{@const blocks = calendarGridTimedBlocks(day, events)}
	{@const draftRange = draftRangeForDay(day)}
	<div
		role="gridcell"
		tabindex="-1"
		data-calendar-date={calendarGridDateKey(day)}
		class="border-border/50 relative flex-1 border-l"
		onpointerdown={(pointerEvent) => handleColumnPointerDown(pointerEvent, day)}
		onpointermove={handleColumnPointerMove}
		onpointerup={handleColumnPointerUp}
		onpointercancel={clearDraft}
	>
		{#each Array.from({ length: 24 }, (_, hour) => hour) as hour (hour)}
			<div class="border-border/50 h-12 border-b"></div>
		{/each}

		{#each blocks as block (block.event.id)}
			{@const blockHeightPixels = Math.max(20, ((block.endMinutes - block.startMinutes) / 60) * hourHeightPixels)}
			<div
				class="absolute px-0.5"
				style={`top: ${(block.startMinutes / 60) * hourHeightPixels}px; height: ${blockHeightPixels}px; left: ${(block.column / block.columnCount) * 100}%; width: ${100 / block.columnCount}%`}
			>
				<CalendarEventChip
					event={block.event}
					size={blockHeightPixels < 44 ? 'compact' : 'block'}
					timeLabel={timeFormatter.format(block.event.start)}
					isSelected={selectedEventID === block.event.id}
					class="h-full"
					{openEvent}
				/>
			</div>
		{/each}

		{#if draftRange}
			<div
				class="bg-primary/20 border-primary pointer-events-none absolute inset-x-0.5 rounded-md border"
				style={`top: ${draftRange.topPixels}px; height: ${draftRange.heightPixels}px`}
			></div>
		{/if}

		{#if isSameCalendarGridDay(day, today)}
			<div class="pointer-events-none absolute inset-x-0 z-10" style={`top: ${(nowMinutes / 60) * hourHeightPixels}px`}>
				<div class="bg-destructive h-px w-full"></div>
			</div>
		{/if}
	</div>
{/snippet}

{#snippet dayHeader(day: Date)}
	<button
		type="button"
		class={cn(
			'flex flex-1 gap-2 text-sm',
			dayCount === 1 ? 'items-end justify-start pt-2 pr-2 pb-1 pl-[22px]' : 'items-center justify-center gap-1.5 px-2 py-2'
		)}
		onclick={() => selectDay(day)}
	>
		{#if dayCount === 1}
			<span
				class={cn(
					'flex size-9 items-center justify-center rounded-full text-2xl leading-none font-semibold tabular-nums',
					isSameCalendarGridDay(day, today) && 'bg-primary text-primary-foreground'
				)}
			>
				{day.getDate()}
			</span>
			<span class="text-muted-foreground pb-1.5">{weekdayFormatter.format(day)}</span>
		{:else}
			<span class="text-muted-foreground">{weekdayFormatter.format(day)}</span>
			<span
				class={cn(
					'flex size-6 items-center justify-center rounded-full font-medium tabular-nums',
					isSameCalendarGridDay(day, today) && 'bg-primary text-primary-foreground'
				)}
			>
				{day.getDate()}
			</span>
		{/if}
	</button>
{/snippet}

{#snippet allDayCell(day: Date)}
	<div class="border-border/50 flex min-h-8 flex-1 flex-col gap-0.5 border-l p-0.5">
		{#each events.filter((event) => event.isAllDay && event.end > startOfCalendarGridDay(day) && event.start < addCalendarGridDays(day, 1)) as event (event.id)}
			<CalendarEventChip {event} isSelected={selectedEventID === event.id} {openEvent} />
		{/each}
	</div>
{/snippet}

<div class="flex min-h-0 flex-1 overflow-hidden">
	{#if canSwipeWeeks}
		<div class="flex min-h-0 flex-1 flex-col">
			<div class="flex shrink-0">
				<div class="bg-background border-border/50 z-30 w-16 shrink-0 border-r">
					<div class="border-border/50 h-9 border-b"></div>
					<div class="border-border/50 text-muted-foreground h-9 border-b-2 px-2 py-1 text-right text-xs whitespace-nowrap">종일</div>
				</div>
				<div bind:this={headerStripElement} class="no-scrollbar min-w-0 flex-1 overflow-x-hidden">
					<div class="flex" style="width: 300%">
						{#each stripDays as day (day.getTime())}
							<div class="flex flex-col" style="width: calc(100% / 21)">
								<div class="border-border/50 flex h-9 items-end border-b">
									{@render dayHeader(day)}
								</div>
								<div class="border-border/50 flex h-9 border-b-2">
									{@render allDayCell(day)}
								</div>
							</div>
						{/each}
					</div>
				</div>
			</div>
			<div
				bind:this={gridElement}
				role="grid"
				tabindex="-1"
				aria-label={weekdayFormatter.format(days[0])}
				class="no-scrollbar min-h-0 flex-1 overflow-y-auto"
				onwheel={handleGridWheel}
				ontouchend={scrollSnap.handleGestureEnd}
				onscrollend={scrollSnap.handleScrollEnd}
			>
				<div class="flex min-w-0">
					<div class="bg-background border-border/50 sticky left-0 z-30 w-16 shrink-0 border-r">
						{#each Array.from({ length: 24 }, (_, hour) => hour) as hour (hour)}
							<div class="text-muted-foreground relative h-12 pr-2 text-right text-xs whitespace-nowrap tabular-nums">
								<span class="absolute top-0 right-2 -translate-y-1/2">
									{hour === 0 ? '' : hourFormatter.format(new Date(2026, 0, 1, hour))}
								</span>
							</div>
						{/each}
					</div>
					<div
						bind:this={columnsElement}
						class="no-scrollbar min-w-0 flex-1 overflow-x-hidden"
						onscroll={syncHeaderStripScroll}
					>
						<div class="flex" style={`width: 300%; height: ${24 * hourHeightPixels}px`}>
							{#each stripDays as day (day.getTime())}
								<div class="flex shrink-0 flex-col" style="width: calc(100% / 21)">
									{@render dayColumn(day)}
								</div>
							{/each}
						</div>
					</div>
				</div>
			</div>
		</div>
	{:else}
		<div class="flex min-h-0 flex-1 flex-col">
			<div class="border-border/50 flex items-end border-b">
				{#each days as day (day.getTime())}
					{@render dayHeader(day)}
				{/each}
			</div>
			<div class="border-border/50 flex border-b-2">
				<div class="text-muted-foreground w-16 shrink-0 px-2 py-1 text-right text-xs whitespace-nowrap">종일</div>
				{#each days as day (day.getTime())}
					{@render allDayCell(day)}
				{/each}
			</div>
			<div
				bind:this={gridElement}
				role="grid"
				tabindex="-1"
				aria-label={weekdayFormatter.format(days[0])}
				class="no-scrollbar min-h-0 flex-1 overflow-y-auto"
				onwheel={scrollSnap.handleWheel}
				ontouchend={scrollSnap.handleGestureEnd}
				onscrollend={scrollSnap.handleScrollEnd}
			>
				<div class="flex" style={`height: ${24 * hourHeightPixels}px`}>
					<div class="w-16 shrink-0">
						{#each Array.from({ length: 24 }, (_, hour) => hour) as hour (hour)}
							<div class="text-muted-foreground relative h-12 pr-2 text-right text-xs whitespace-nowrap tabular-nums">
								<span class="absolute top-0 right-2 -translate-y-1/2">
									{hour === 0 ? '' : hourFormatter.format(new Date(2026, 0, 1, hour))}
								</span>
							</div>
						{/each}
					</div>
					{#each days as day (day.getTime())}
						{@render dayColumn(day)}
					{/each}
				</div>
			</div>
		</div>
	{/if}

	{#if dayCount === 1}
		<aside class="border-border/50 hidden w-fit shrink-0 border-l p-2 lg:block">
			<MiniCalendar type="single" value={miniCalendarValue} onValueChange={selectMiniCalendarDate} locale={localeCode} />
		</aside>
	{/if}
</div>
