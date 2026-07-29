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

	let gridElement = $state<HTMLElement | null>(null);
	let draftStartMinutes = $state<number | null>(null);
	let draftEndMinutes = $state<number | null>(null);
	let draftDay = $state<Date | null>(null);
	let longPressTimer: number | null = null;

	const days = $derived(
		dayCount === 1
			? [startOfCalendarGridDay(visibleDate)]
			: Array.from({ length: dayCount }, (_, dayOffset) => addCalendarGridDays(startOfCalendarGridWeek(visibleDate), dayOffset))
	);
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

	$effect(() => {
		return () => {
			if (longPressTimer !== null) clearTimeout(longPressTimer);
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

<div class="flex min-h-0 flex-1">
<div class="flex min-h-0 flex-1 flex-col">
	<div class="border-border/70 flex border-b">
		<div class="w-16 shrink-0"></div>
		{#each days as day (day.getTime())}
			<button
				type="button"
				class="flex flex-1 items-center justify-center gap-1.5 px-2 py-2 text-sm"
				onclick={() => selectDay(day)}
			>
				<span class="text-muted-foreground">{weekdayFormatter.format(day)}</span>
				<span
					class={cn(
						'flex size-6 items-center justify-center rounded-full font-medium tabular-nums',
						isSameCalendarGridDay(day, today) && 'bg-primary text-primary-foreground'
					)}
				>
					{day.getDate()}
				</span>
			</button>
		{/each}
	</div>

	<div class="border-border/70 flex border-b">
		<div class="text-muted-foreground w-16 shrink-0 px-2 py-1 text-right text-xs whitespace-nowrap">종일</div>
		{#each allDayEvents as dayEntry (dayEntry.day.getTime())}
			<div class="border-border/70 flex min-h-8 flex-1 flex-col gap-0.5 border-l p-0.5">
				{#each dayEntry.events as event (event.id)}
					<CalendarEventChip {event} isSelected={selectedEventID === event.id} {openEvent} />
				{/each}
			</div>
		{/each}
	</div>

	<div bind:this={gridElement} class="min-h-0 flex-1 overflow-y-auto">
		<div class="flex" style={`height: ${24 * hourHeightPixels}px`}>
			<div class="w-16 shrink-0">
				{#each Array.from({ length: 24 }, (_, hour) => hour) as hour (hour)}
					<div class="text-muted-foreground relative h-12 pr-2 text-right text-xs whitespace-nowrap tabular-nums">
						<span class="absolute top-0 right-2 -translate-y-1/2">{hour === 0 ? '' : hourFormatter.format(new Date(2026, 0, 1, hour))}</span>
					</div>
				{/each}
			</div>
			{#each days as day (day.getTime())}
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
			{/each}
		</div>
	</div>
</div>
	{#if dayCount === 1}
		<aside class="border-border/70 hidden w-72 shrink-0 border-l p-3 lg:block">
			<MiniCalendar
				type="single"
				value={miniCalendarValue}
				onValueChange={selectMiniCalendarDate}
				locale={localeCode}
				class="w-full"
			/>
		</aside>
	{/if}
</div>
