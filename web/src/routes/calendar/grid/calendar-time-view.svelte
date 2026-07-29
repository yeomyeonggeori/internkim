<script lang="ts">
	import { Calendar as MiniCalendar, Day as MiniCalendarDay } from '$lib/components/ui/calendar';
	import { cn } from '$lib/utils';
	import { CalendarDate, type DateValue } from '@internationalized/date';
	import { tick } from 'svelte';
	import CalendarEventChip from './calendar-event-chip.svelte';
	import CalendarEventListCard from '../embed/calendar-event-list-card.svelte';
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
	import { defaultCalendarEventColor } from './calendar-grid-events';
	import { calendarGridAllDaySpans, calendarGridTimedBlocks, type CalendarGridEvent } from './calendar-grid-layout';
	import { createCalendarScrollSnap } from './calendar-grid-scroll-snap.svelte';

	type CalendarTimeViewProps = {
		visibleDate: Date;
		dayCount: number;
		selectedEventID: string;
		events: CalendarGridEvent[];
		localeCode: string;
		draftPreviewTitle: string;
		selectDay: (day: Date) => void;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		addEventOnTimeRange: (start: Date, end: Date) => void;
		addEventOnDayRange: (startDateKey: string, endDateKey: string) => void;
	};

	let {
		visibleDate,
		dayCount,
		selectedEventID,
		events,
		localeCode,
		draftPreviewTitle,
		selectDay,
		openEvent,
		addEventOnTimeRange,
		addEventOnDayRange
	}: CalendarTimeViewProps = $props();

	const hourHeightPixels = 48;
	const minutesPerDay = 24 * 60;
	const dragSnapMinutes = 15;
	const longPressMilliseconds = 450;
	const defaultDurationMinutes = 60;
	const flickVelocityPixelsPerMillisecond = 0.3;
	const flickTravelPixels = 8;
	const minimumColumnTravelRatio = 0.25;

	let gridElement = $state<HTMLElement | null>(null);
	let draftStart = $state<{ day: Date; minutes: number } | null>(null);
	let draftEnd = $state<{ day: Date; minutes: number } | null>(null);
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
	const allDayRowLabel = '종일';
	const allDayLaneHeightPixels = 22;
	let allDayDragColumns = $state<{ days: Date[]; startIndex: number; endIndex: number } | null>(null);

	const allDayDragSpan = $derived(
		allDayDragColumns
			? {
					startColumn: Math.min(allDayDragColumns.startIndex, allDayDragColumns.endIndex),
					columnCount: Math.abs(allDayDragColumns.endIndex - allDayDragColumns.startIndex) + 1
				}
			: null
	);

	function allDayColumnIndexFromPointer(event: PointerEvent, columnCount: number): number {
		const rowElement = event.currentTarget;
		if (!(rowElement instanceof HTMLElement)) return 0;
		const bounds = rowElement.getBoundingClientRect();
		const ratio = (event.clientX - bounds.left) / Math.max(1, bounds.width);
		return Math.min(columnCount - 1, Math.max(0, Math.floor(ratio * columnCount)));
	}

	function startAllDayDrag(event: PointerEvent, columnDays: Date[]): void {
		if (event.button !== 0) return;
		const columnIndex = allDayColumnIndexFromPointer(event, columnDays.length);
		allDayDragColumns = { days: columnDays, startIndex: columnIndex, endIndex: columnIndex };
	}

	function extendAllDayDrag(event: PointerEvent, columnDays: Date[]): void {
		if (!allDayDragColumns) return;
		allDayDragColumns = { ...allDayDragColumns, endIndex: allDayColumnIndexFromPointer(event, columnDays.length) };
	}

	function finishAllDayDrag(): void {
		const drag = allDayDragColumns;
		allDayDragColumns = null;
		if (!drag) return;
		const firstIndex = Math.min(drag.startIndex, drag.endIndex);
		const lastIndex = Math.max(drag.startIndex, drag.endIndex);
		const startDay = drag.days[firstIndex];
		const endDay = drag.days[lastIndex];
		if (!startDay || !endDay) return;
		addEventOnDayRange(calendarGridDateKey(startDay), calendarGridDateKey(endDay));
	}
	const stripAllDaySpans = $derived(calendarGridAllDaySpans(stripDays, events));
	const daysAllDaySpans = $derived(calendarGridAllDaySpans(days, events));

	function allDayRowHeightPixels(spans: { lane: number }[]): number {
		const laneCount = spans.reduce((count, span) => Math.max(count, span.lane + 1), 1);
		return laneCount * allDayLaneHeightPixels + 8;
	}
	const weekdayFormatter = $derived(new Intl.DateTimeFormat(localeCode, { weekday: 'short' }));
	const hourFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric' }));
	const timeFormatter = $derived(new Intl.DateTimeFormat(localeCode, { hour: 'numeric', minute: '2-digit' }));
	const today = new Date();
	const draftEventPreview = $derived<CalendarGridEvent>({
		id: 'calendar-draft-preview',
		title: draftPreviewTitle,
		start: today,
		end: today,
		isAllDay: false,
		color: defaultCalendarEventColor
	});
	const draftAllDayPreview = $derived<CalendarGridEvent>({
		id: 'calendar-draft-all-day-preview',
		title: draftPreviewTitle,
		start: today,
		end: today,
		isAllDay: true,
		color: defaultCalendarEventColor
	});
	const nowMinutes = $derived(calendarGridMinutesFromMidnight(today));
	const miniCalendarValue = $derived(
		new CalendarDate(visibleDate.getFullYear(), visibleDate.getMonth() + 1, visibleDate.getDate()) as DateValue
	);

	function selectMiniCalendarDate(dateValue: DateValue | undefined): void {
		if (!dateValue) return;
		selectDay(new Date(dateValue.year, dateValue.month - 1, dateValue.day, 12, 0, 0, 0));
	}

	const eventDateKeys = $derived(new Set(events.flatMap(calendarGridEventDateKeys)));

	function calendarGridEventDateKeys(event: CalendarGridEvent): string[] {
		const lastDay = startOfCalendarGridDay(new Date(Math.max(event.end.getTime() - 1, event.start.getTime())));
		const dateKeys: string[] = [];
		for (let day = startOfCalendarGridDay(event.start); day <= lastDay; day = addCalendarGridDays(day, 1)) {
			dateKeys.push(calendarGridDateKey(day));
		}
		return dateKeys;
	}

	const selectedDayEvents = $derived(
		events
			.filter((event) => event.end > startOfCalendarGridDay(visibleDate) && event.start < addCalendarGridDays(startOfCalendarGridDay(visibleDate), 1))
			.sort((first, second) => Number(second.isAllDay) - Number(first.isAllDay) || first.start.getTime() - second.start.getTime())
	);

	function eventTimeLabel(event: CalendarGridEvent): string {
		if (event.isAllDay) return '';
		return `${clockLabel(event.start)}-${clockLabel(event.end)}`;
	}

	function clockLabel(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}

	function hasEventsOnDate(dateValue: DateValue): boolean {
		return eventDateKeys.has(calendarGridDateKey(new Date(dateValue.year, dateValue.month - 1, dateValue.day)));
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
		void recenterColumns();
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
		gestureVelocity
	}: {
		currentOffset: number;
		gestureStartOffset: number;
		offsets: number[];
		gestureVelocity: number;
	}): number {
		const columnWidth = columnWidthPixels();
		if (columnWidth === 0 || offsets.length === 0) return currentOffset;
		const travel = currentOffset - gestureStartOffset;
		if (Math.abs(travel) < flickTravelPixels) return gestureStartOffset;
		const direction = travel > 0 ? 1 : -1;
		const startColumnIndex = Math.round(gestureStartOffset / columnWidth);
		const targetColumnIndex =
			gestureVelocity >= flickVelocityPixelsPerMillisecond
				? weekAlignedColumnIndex(startColumnIndex, direction)
				: nearestColumnIndexForTravel(currentOffset, travel, columnWidth, startColumnIndex);
		const boundedIndex = Math.min(offsets.length - 1, Math.max(0, targetColumnIndex));
		return boundedIndex * columnWidth;
	}

	function nearestColumnIndexForTravel(
		currentOffset: number,
		travel: number,
		columnWidth: number,
		startColumnIndex: number
	): number {
		const nearestIndex = Math.round(currentOffset / columnWidth);
		if (Math.abs(travel) < columnWidth * minimumColumnTravelRatio) return nearestIndex;
		const steppedIndex = startColumnIndex + (travel > 0 ? 1 : -1);
		return travel > 0 ? Math.max(nearestIndex, steppedIndex) : Math.min(nearestIndex, steppedIndex);
	}

	function weekAlignedColumnIndex(startColumnIndex: number, direction: number): number {
		const leadingDay = stripDays[Math.min(stripDays.length - 1, Math.max(0, startColumnIndex))] ?? weekWindowStart;
		const leadingWeekStart = startOfCalendarGridWeek(leadingDay);
		const isWeekAligned = isSameCalendarGridDay(leadingDay, leadingWeekStart);
		const targetDay =
			direction < 0 && !isWeekAligned ? leadingWeekStart : addCalendarGridDays(leadingWeekStart, direction * 7);
		const dayOffset = Math.round((targetDay.getTime() - weekWindowStart.getTime()) / 86400000);
		return 7 + dayOffset;
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
		selectDay(leadingDay);
		void recenterColumns();
	}

	function syncHeaderStripScroll(): void {
		if (!columnsElement || !headerStripElement) return;
		headerStripElement.scrollLeft = columnsElement.scrollLeft;
	}

	async function recenterColumns(): Promise<void> {
		isRecenteringColumns = true;
		await tick();
		if (columnsElement) columnsElement.scrollLeft = columnWidthPixels() * 7;
		syncHeaderStripScroll();
		requestAnimationFrame(() => {
			isRecenteringColumns = false;
		});
	}

	$effect(() => {
		if (!canSwipeWeeks || !columnsElement) return;
		weekWindowStartKey;
		void recenterColumns();
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

	function draftPointFromEvent(pointerEvent: PointerEvent): { day: Date; minutes: number } | null {
		const element = columnsElement?.ownerDocument.elementFromPoint(pointerEvent.clientX, pointerEvent.clientY);
		const dayColumn = element?.closest<HTMLElement>('[data-calendar-date]');
		const dateKey = dayColumn?.dataset.calendarDate;
		if (!dayColumn || !dateKey) return null;
		return { day: calendarGridDateFromKey(dateKey), minutes: minutesFromPoint(dayColumn, pointerEvent.clientY) };
	}

	function handleColumnPointerDown(pointerEvent: PointerEvent, day: Date): void {
		if (pointerEvent.button !== 0 || !(pointerEvent.currentTarget instanceof HTMLElement)) return;
		if (pointerEvent.target instanceof Element && pointerEvent.target.closest('button')) return;
		pointerEvent.preventDefault();
		const startMinutes = minutesFromPoint(pointerEvent.currentTarget, pointerEvent.clientY);
		draftStart = { day, minutes: startMinutes };
		draftEnd = { day, minutes: startMinutes };
		capturePointer(pointerEvent);
		longPressTimer = window.setTimeout(() => {
			longPressTimer = null;
			if (!draftStart || !draftEnd) return;
			if (draftEnd.minutes !== draftStart.minutes || !isSameCalendarGridDay(draftEnd.day, draftStart.day)) return;
			commitDraft({ day: draftStart.day, minutes: draftStart.minutes + defaultDurationMinutes });
		}, longPressMilliseconds);
	}

	function capturePointer(pointerEvent: PointerEvent): void {
		if (!(pointerEvent.currentTarget instanceof HTMLElement)) return;
		if (!pointerEvent.isTrusted) return;
		pointerEvent.currentTarget.setPointerCapture(pointerEvent.pointerId);
	}

	function handleColumnPointerMove(pointerEvent: PointerEvent): void {
		if (!draftStart) return;
		const draftPoint = draftPointFromEvent(pointerEvent);
		if (!draftPoint) return;
		draftEnd = draftPoint;
	}

	function handleColumnPointerUp(): void {
		if (!draftStart || !draftEnd) return;
		commitDraft(draftEnd);
	}

	function commitDraft(endPoint: { day: Date; minutes: number }): void {
		cancelLongPress();
		const startPoint = draftStart;
		clearDraft();
		if (!startPoint) return;
		const startDate = calendarGridDateAtMinutes(startPoint.day, startPoint.minutes);
		const endDate = calendarGridDateAtMinutes(endPoint.day, endPoint.minutes);
		const [firstDate, lastDate] = [startDate, endDate].sort((first, second) => first.getTime() - second.getTime());
		const rangeEndDate =
			lastDate.getTime() - firstDate.getTime() < dragSnapMinutes * 60000
				? new Date(firstDate.getTime() + defaultDurationMinutes * 60000)
				: lastDate;
		addEventOnTimeRange(firstDate, rangeEndDate);
	}

	function cancelLongPress(): void {
		if (longPressTimer === null) return;
		clearTimeout(longPressTimer);
		longPressTimer = null;
	}

	function clearDraft(): void {
		cancelLongPress();
		draftStart = null;
		draftEnd = null;
	}

	function draftRangeForDay(day: Date): { topPixels: number; heightPixels: number } | null {
		if (!draftStart || !draftEnd) return null;
		const startDate = calendarGridDateAtMinutes(draftStart.day, draftStart.minutes);
		const endDate = calendarGridDateAtMinutes(draftEnd.day, draftEnd.minutes);
		const [firstDate, lastDate] = [startDate, endDate].sort((first, second) => first.getTime() - second.getTime());
		const dayStart = startOfCalendarGridDay(day);
		const dayEnd = addCalendarGridDays(dayStart, 1);
		if (lastDate <= dayStart || firstDate >= dayEnd) return null;
		const startMinutes = firstDate <= dayStart ? 0 : calendarGridMinutesFromMidnight(firstDate);
		const endMinutes = lastDate >= dayEnd ? minutesPerDay : calendarGridMinutesFromMidnight(lastDate);
		return {
			topPixels: (startMinutes / 60) * hourHeightPixels,
			heightPixels: Math.max(hourHeightPixels / 4, ((endMinutes - startMinutes) / 60) * hourHeightPixels)
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
					placeholder={draftPreviewTitle}
					class="h-full"
					{openEvent}
				/>
			</div>
		{/each}

		{#if draftRange}
			<div
				class="pointer-events-none absolute inset-x-0.5"
				style={`top: ${draftRange.topPixels}px; height: ${draftRange.heightPixels}px`}
			>
				<CalendarEventChip
					event={draftEventPreview}
					size={draftRange.heightPixels < 44 ? 'compact' : 'block'}
					isSelected
					class="h-full"
					openEvent={() => {}}
				/>
			</div>
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

{#snippet allDayRow(columnDays: Date[], spans: ReturnType<typeof calendarGridAllDaySpans>)}
	<div
		role="grid"
		tabindex="-1"
		aria-label={allDayRowLabel}
		class="relative select-none"
		style={`height: ${allDayRowHeightPixels(spans)}px`}
		onpointerdown={(event) => startAllDayDrag(event, columnDays)}
		onpointermove={(event) => extendAllDayDrag(event, columnDays)}
		onpointerup={finishAllDayDrag}
		onpointerleave={() => (allDayDragColumns = null)}
	>
		<div class="absolute inset-0 grid" style={`grid-template-columns: repeat(${columnDays.length}, minmax(0, 1fr))`}>
			{#each columnDays as day (day.getTime())}
				<div class="border-border/50 border-l"></div>
			{/each}
		</div>
		<div class="absolute inset-0 grid px-0.5 pt-1" style={`grid-template-columns: repeat(${columnDays.length}, minmax(0, 1fr))`}>
			{#if allDayDragSpan && allDayDragColumns?.days.length === columnDays.length}
				<div
					class="px-0.5"
					style={`grid-column: ${allDayDragSpan.startColumn + 1} / span ${allDayDragSpan.columnCount}; grid-row: 1`}
				>
					<CalendarEventChip event={draftAllDayPreview} isSelected openEvent={() => {}} />
				</div>
			{/if}
			{#each spans as span (span.event.id)}
				<div
					class="px-0.5"
					style={`grid-column: ${span.startColumn + 1} / span ${span.columnCount}; grid-row: 1; margin-top: ${span.lane * allDayLaneHeightPixels}px`}
				>
					<CalendarEventChip
						event={span.event}
						isSelected={selectedEventID === span.event.id}
						continuesBefore={span.continuesBefore}
						continuesAfter={span.continuesAfter}
						placeholder={draftPreviewTitle}
						{openEvent}
					/>
				</div>
			{/each}
		</div>
	</div>
{/snippet}

<div class="flex min-h-0 flex-1 overflow-hidden">
	{#if canSwipeWeeks}
		<div class="flex min-h-0 flex-1 flex-col">
			<div class="flex shrink-0">
				<div class="bg-background border-border/50 z-30 w-16 shrink-0 border-r">
					<div class="border-border/50 h-9 border-b"></div>
					<div
						class="border-border/50 text-muted-foreground border-b-2 px-2 py-1 text-right text-xs whitespace-nowrap"
						style={`height: ${allDayRowHeightPixels(stripAllDaySpans)}px`}
					>
						종일
					</div>
				</div>
				<div bind:this={headerStripElement} class="no-scrollbar min-w-0 flex-1 overflow-x-hidden">
					<div style="width: 300%">
						<div class="flex">
							{#each stripDays as day (day.getTime())}
								<div class="border-border/50 flex h-9 items-end border-b" style="width: calc(100% / 21)">
									{@render dayHeader(day)}
								</div>
							{/each}
						</div>
						<div class="border-border/50 border-b-2">
							{@render allDayRow(stripDays, stripAllDaySpans)}
						</div>
					</div>
				</div>
			</div>
			<div
				bind:this={gridElement}
				role="grid"
				tabindex="-1"
				aria-label={weekdayFormatter.format(days[0])}
				class="no-scrollbar min-h-0 flex-1 select-none overflow-y-auto"
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
				<div class="min-w-0 flex-1">
					{@render allDayRow(days, daysAllDaySpans)}
				</div>
			</div>
			<div
				bind:this={gridElement}
				role="grid"
				tabindex="-1"
				aria-label={weekdayFormatter.format(days[0])}
				class="no-scrollbar min-h-0 flex-1 select-none overflow-y-auto"
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
			<MiniCalendar type="single" value={miniCalendarValue} onValueChange={selectMiniCalendarDate} locale={localeCode}>
				{#snippet day({ day })}
					<MiniCalendarDay>
						{day.day}
						{#if hasEventsOnDate(day)}
							<span class="size-1 rounded-full !opacity-100" style={`background: ${defaultCalendarEventColor}`}></span>
						{/if}
					</MiniCalendarDay>
				{/snippet}
			</MiniCalendar>
			{#if selectedDayEvents.length > 0}
				<div class="mt-3 grid gap-2">
					{#each selectedDayEvents as event (event.id)}
						<CalendarEventListCard
							title={event.title}
							color={event.color}
							start={event.start}
							isAllDay={event.isAllDay}
							timeLabel={eventTimeLabel(event)}
							openEvent={(originElement) => openEvent(event, originElement)}
						/>
					{/each}
				</div>
			{/if}
		</aside>
	{/if}
</div>
