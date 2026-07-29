<script lang="ts">
	import { Calendar as MiniCalendar } from '$lib/components/ui/calendar';
	import { cn } from '$lib/utils';
	import { CalendarDate, type DateValue } from '@internationalized/date';
	import CalendarEventChip from './calendar-event-chip.svelte';
	import {
		addCalendarGridDays,
		calendarGridDateAtMinutes,
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
	const weekSwipeThresholdPixels = 120;
	const weekSwipeGapMilliseconds = 150;

	let gridElement = $state<HTMLElement | null>(null);
	let draftStartMinutes = $state<number | null>(null);
	let draftEndMinutes = $state<number | null>(null);
	let draftDay = $state<Date | null>(null);
	let longPressTimer: number | null = null;
	let horizontalTravel = 0;
	let lastHorizontalWheelTime = 0;
	let hasSwipedThisGesture = false;
	let pendingSlideDirection = 0;
	let weekSlidePercent = $state(0);
	let isWeekSlideAnimated = $state(false);
	let slideFrame: number | null = null;

	const days = $derived(
		dayCount === 1
			? [startOfCalendarGridDay(visibleDate)]
			: Array.from({ length: dayCount }, (_, dayOffset) => addCalendarGridDays(startOfCalendarGridWeek(visibleDate), dayOffset))
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








	function handleHorizontalWheel(wheelEvent: WheelEvent): void {
		if (!canSwipeWeeks || Math.abs(wheelEvent.deltaX) <= Math.abs(wheelEvent.deltaY)) {
			scrollSnap.handleWheel(wheelEvent);
			return;
		}
		wheelEvent.preventDefault();
		const now = performance.now();
		if (now - lastHorizontalWheelTime > weekSwipeGapMilliseconds) horizontalTravel = 0;
		lastHorizontalWheelTime = now;
		if (hasSwipedThisGesture) return;
		horizontalTravel += wheelEvent.deltaX;
		if (Math.abs(horizontalTravel) < weekSwipeThresholdPixels) return;
		const weekDirection = horizontalTravel > 0 ? 7 : -7;
		hasSwipedThisGesture = true;
		horizontalTravel = 0;
		pendingSlideDirection = weekDirection > 0 ? 1 : -1;
		selectDay(addCalendarGridDays(startOfCalendarGridWeek(visibleDate), weekDirection));
	}

	function hourScrollOffsets(): number[] {
		return Array.from({ length: 24 }, (_, hour) => hour * hourHeightPixels);
	}

	$effect(() => {
		visibleDate;
		hasSwipedThisGesture = false;
		horizontalTravel = 0;
		if (pendingSlideDirection === 0) return;
		startWeekSlide(pendingSlideDirection);
		pendingSlideDirection = 0;
	});

	function startWeekSlide(direction: number): void {
		if (slideFrame !== null) cancelAnimationFrame(slideFrame);
		isWeekSlideAnimated = false;
		weekSlidePercent = direction * 100;
		slideFrame = requestAnimationFrame(() => {
			slideFrame = requestAnimationFrame(() => {
				slideFrame = null;
				isWeekSlideAnimated = true;
				weekSlidePercent = 0;
			});
		});
	}

	$effect(() => {
		return () => {
			if (longPressTimer !== null) clearTimeout(longPressTimer);
			if (slideFrame !== null) cancelAnimationFrame(slideFrame);
			scrollSnap.destroy();
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
		class="border-border/70 relative flex-1 border-l"
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
	<div class="border-border/70 flex min-h-8 flex-1 flex-col gap-0.5 border-l p-0.5">
		{#each events.filter((event) => event.isAllDay && event.end > startOfCalendarGridDay(day) && event.start < addCalendarGridDays(day, 1)) as event (event.id)}
			<CalendarEventChip {event} isSelected={selectedEventID === event.id} {openEvent} />
		{/each}
	</div>
{/snippet}

<div class="flex min-h-0 flex-1 overflow-hidden">
		<div
			class="flex min-h-0 flex-1 flex-col"
			style={`transform: translateX(${weekSlidePercent}%); transition: transform ${isWeekSlideAnimated ? '220ms cubic-bezier(0.22, 1, 0.36, 1)' : '0ms'}`}
		>
			<div class="border-border/70 flex items-end border-b">
				{#each days as day (day.getTime())}
					{@render dayHeader(day)}
				{/each}
			</div>
			<div class="border-border/70 flex border-b">
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
				onwheel={handleHorizontalWheel}
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

	{#if dayCount === 1}
		<aside class="border-border/70 hidden w-fit shrink-0 border-l p-2 lg:block">
			<MiniCalendar type="single" value={miniCalendarValue} onValueChange={selectMiniCalendarDate} locale={localeCode} />
		</aside>
	{/if}
</div>
