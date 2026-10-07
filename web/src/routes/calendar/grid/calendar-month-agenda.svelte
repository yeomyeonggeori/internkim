<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import { calendarParticipantKey } from '../embed/calendar-participants';
	import { addCalendarGridDays, calendarGridDateKey, isSameCalendarGridDay } from './calendar-grid-dates';
	import type { CalendarGridEvent } from './calendar-grid-layout';
	import { calendarGridEventsOnDay, calendarGridMonthWeeks, shiftedCalendarGridMonthDay } from './calendar-month-agenda-days';
	import type { CalendarVisibleDateRange } from './calendar-visible-event-range';

	type CalendarMonthAgendaProps = {
		visibleDate: Date;
		events: CalendarGridEvent[];
		selectedEventID: string;
		localeCode: string;
		loading: boolean;
		allDayText: string;
		noEventsText: string;
		untitledText: string;
		selectDay: (day: Date) => void;
		addEventOnDay: (day: Date) => void;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		visibleRangeChanged: (range: CalendarVisibleDateRange) => void;
	};

	let {
		visibleDate,
		events,
		selectedEventID,
		localeCode,
		loading,
		allDayText,
		noEventsText,
		untitledText,
		selectDay,
		addEventOnDay,
		openEvent,
		visibleRangeChanged
	}: CalendarMonthAgendaProps = $props();

	const markerLimit = 3;
	const participantLimit = 3;
	const swipeDistancePixels = 48;
	const today = new Date();

	const weeks = $derived(calendarGridMonthWeeks(visibleDate));
	const weekdayLabels = $derived(
		Array.from({ length: 7 }, (_, weekdayIndex) =>
			new Date(2026, 2, 1 + weekdayIndex).toLocaleDateString(localeCode, { weekday: 'narrow' })
		)
	);
	const dayLabelFormatter = $derived(new Intl.DateTimeFormat(localeCode, { month: 'long', day: 'numeric', weekday: 'long' }));
	const selectedDayEvents = $derived(calendarGridEventsOnDay(events, visibleDate));

	let swipeStart: { clientX: number; clientY: number } | null = null;
	let isSwipeClick = false;

	$effect(() => {
		const lastWeek = weeks.at(-1);
		if (!lastWeek) return;
		visibleRangeChanged({ start: weeks[0].days[0], end: addCalendarGridDays(lastWeek.days[0], 7) });
	});

	function isOutsideVisibleMonth(day: Date): boolean {
		return day.getMonth() !== visibleDate.getMonth() || day.getFullYear() !== visibleDate.getFullYear();
	}

	function isRedDate(day: Date, dayEvents: CalendarGridEvent[]): boolean {
		return day.getDay() === 0 || day.getDay() === 6 || dayEvents.some((event) => event.readOnly);
	}

	function markerColors(dayEvents: CalendarGridEvent[]): string[] {
		return dayEvents
			.filter((event) => !event.readOnly)
			.slice(0, markerLimit)
			.map((event) => event.color);
	}

	function clockLabel(date: Date): string {
		return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
	}

	function startSwipe(pointerEvent: PointerEvent): void {
		swipeStart = { clientX: pointerEvent.clientX, clientY: pointerEvent.clientY };
		isSwipeClick = false;
	}

	function finishSwipe(pointerEvent: PointerEvent): void {
		if (!swipeStart) return;
		const horizontalDistance = pointerEvent.clientX - swipeStart.clientX;
		const verticalDistance = pointerEvent.clientY - swipeStart.clientY;
		swipeStart = null;
		if (Math.abs(horizontalDistance) < swipeDistancePixels || Math.abs(horizontalDistance) < Math.abs(verticalDistance) * 1.5) return;
		isSwipeClick = true;
		selectDay(shiftedCalendarGridMonthDay(visibleDate, horizontalDistance < 0 ? 1 : -1));
	}

	function selectDayUnlessSwiped(day: Date): void {
		if (isSwipeClick) {
			isSwipeClick = false;
			return;
		}
		selectDay(day);
	}
</script>

{#snippet eventTime(event: CalendarGridEvent)}
	<span class="text-muted-foreground flex w-11 shrink-0 flex-col text-xs leading-4 tabular-nums">
		{#if event.isAllDay}
			{allDayText}
		{:else}
			<span class="text-foreground">{clockLabel(event.start)}</span>
			<span>{clockLabel(event.end)}</span>
		{/if}
	</span>
{/snippet}

{#snippet eventBody(event: CalendarGridEvent)}
	{@render eventTime(event)}
	<span class="w-[3px] shrink-0 self-stretch rounded-full bg-(--calendar-event-color)"></span>
	<span class={cn('min-w-0 flex-1 truncate text-sm font-medium', !event.title && 'text-muted-foreground')}>
		{event.title || untitledText}
	</span>
	{#if event.participants.length > 0}
		<span class="flex shrink-0 items-center -space-x-1">
			{#each event.participants.slice(0, participantLimit) as participant (calendarParticipantKey(participant))}
				<PersonAvatar
					name={displayPersonName(participant.name)}
					email={participant.email ?? ''}
					seed={calendarParticipantKey(participant)}
					image={participant.image ?? ''}
					class="ring-background size-5 ring-2"
				/>
			{/each}
			{#if event.participants.length > participantLimit}
				<span class="text-muted-foreground pl-2 text-xs tabular-nums">+{event.participants.length - participantLimit}</span>
			{/if}
		</span>
	{/if}
{/snippet}

<div class="flex min-h-0 flex-1 flex-col">
	<div class="text-muted-foreground grid grid-cols-7 pt-1.5 text-center text-xs font-medium">
		{#each weekdayLabels as weekdayLabel, weekdayIndex (weekdayIndex)}
			<div class={cn('py-1', (weekdayIndex === 0 || weekdayIndex === 6) && 'text-destructive')}>{weekdayLabel}</div>
		{/each}
	</div>
	<div
		role="grid"
		tabindex="-1"
		aria-label={visibleDate.toLocaleDateString(localeCode, { year: 'numeric', month: 'long' })}
		class="shrink-0 touch-pan-y pb-1 select-none"
		onpointerdown={startSwipe}
		onpointerup={finishSwipe}
		onpointercancel={() => (swipeStart = null)}
	>
		{#each weeks as week (week.startDateKey)}
			<div role="row" class="grid grid-cols-7">
				{#each week.days as day (day.getTime())}
					{@const dayEvents = calendarGridEventsOnDay(events, day)}
					{@const isToday = isSameCalendarGridDay(day, today)}
					{@const isSelected = isSameCalendarGridDay(day, visibleDate)}
					{@const isRed = isRedDate(day, dayEvents)}
					<div role="gridcell" aria-selected={isSelected} data-calendar-date={calendarGridDateKey(day)}>
						<button
							type="button"
							aria-label={dayLabelFormatter.format(day)}
							aria-current={isToday ? 'date' : undefined}
							class="focus-visible:ring-ring/50 flex h-12 w-full flex-col items-center gap-1 pt-1 outline-none focus-visible:ring-2 focus-visible:ring-inset"
							onclick={() => selectDayUnlessSwiped(day)}
							ondblclick={() => addEventOnDay(day)}
						>
							<span
								class={cn(
									'flex size-7 items-center justify-center rounded-full text-sm tabular-nums',
									isOutsideVisibleMonth(day) && !isSelected && 'opacity-40',
									isRed && !isToday && 'text-destructive',
									isSelected && !isToday && 'bg-muted font-semibold',
									isToday && !isRed && 'bg-primary text-primary-foreground font-semibold',
									isToday && isRed && 'bg-destructive text-background font-semibold'
								)}
							>
								{day.getDate()}
							</span>
							<span class="flex h-1 items-center gap-[3px]" aria-hidden="true">
								{#each markerColors(dayEvents) as color, markerIndex (markerIndex)}
									<span class="size-1 rounded-full" style={`background: ${color}`}></span>
								{/each}
							</span>
						</button>
					</div>
				{/each}
			</div>
		{/each}
	</div>
	<section class="border-border/50 flex min-h-0 flex-1 flex-col border-t" aria-label={dayLabelFormatter.format(visibleDate)}>
		<h2 class="px-4 pt-3 pb-1 text-sm font-semibold">{dayLabelFormatter.format(visibleDate)}</h2>
		<div class="min-h-0 flex-1 overflow-y-auto pb-24">
			{#if loading}
				<div class="grid gap-1 px-4 py-2" aria-hidden="true" data-calendar-agenda-loading>
					{#each [0, 1] as row (row)}
						<div class="flex items-center gap-3 py-2"><Skeleton class="h-3 w-11" /><Skeleton class="h-4 flex-1" /></div>
					{/each}
				</div>
			{:else if selectedDayEvents.length === 0}
				<p class="text-muted-foreground px-4 py-2 text-sm">{noEventsText}</p>
			{:else}
				<ul class="grid">
					{#each selectedDayEvents as event (event.id)}
						<li style={`--calendar-event-color: ${event.color}`}>
							{#if event.readOnly}
								<div class="flex min-h-12 items-center gap-3 px-4 py-2">{@render eventBody(event)}</div>
							{:else}
								<button
									type="button"
									data-calendar-event-id={event.id}
									data-selected={selectedEventID === event.id ? '' : undefined}
									class={cn(
										'hover:bg-muted/60 focus-visible:ring-ring/50 flex min-h-12 w-full items-center gap-3 px-4 py-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-inset',
										selectedEventID === event.id && 'bg-muted'
									)}
									onclick={(clickEvent) => openEvent(event, clickEvent.currentTarget)}
								>
									{@render eventBody(event)}
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</section>
</div>
