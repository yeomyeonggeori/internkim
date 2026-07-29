<script lang="ts">
	import { cn } from '$lib/utils';
	import type { CalendarGridEvent } from './calendar-grid-layout';

	type CalendarEventChipProps = {
		event: CalendarGridEvent;
		variant?: 'timed' | 'span';
		timeLabel?: string;
		isSelected?: boolean;
		continuesBefore?: boolean;
		continuesAfter?: boolean;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		class?: string;
	};

	let {
		event,
		variant = 'timed',
		timeLabel = '',
		isSelected = false,
		continuesBefore = false,
		continuesAfter = false,
		openEvent,
		class: className
	}: CalendarEventChipProps = $props();
</script>

<button
	type="button"
	data-calendar-event-id={event.id}
	data-selected={isSelected ? '' : undefined}
	style={`--calendar-event-color: ${event.color}`}
	class={cn(
		'flex w-full min-w-0 items-center gap-1.5 overflow-hidden text-left text-xs leading-4 transition-colors',
		'focus-visible:ring-ring/50 outline-none focus-visible:ring-2',
		variant === 'timed' && 'rounded-sm px-1.5 py-0.5 hover:bg-(--calendar-event-color)/15',
		variant === 'span' &&
			'bg-(--calendar-event-color)/15 text-foreground h-5 rounded-sm px-1.5 font-medium hover:bg-(--calendar-event-color)/25',
		continuesBefore && 'rounded-l-none',
		continuesAfter && 'rounded-r-none',
		isSelected && 'ring-(--calendar-event-color) ring-2',
		className
	)}
	onclick={(clickEvent) => openEvent(event, clickEvent.currentTarget)}
>
	{#if variant === 'timed'}
		<span class="size-1.5 shrink-0 rounded-full bg-(--calendar-event-color)"></span>
	{/if}
	<span class="truncate font-medium">{event.title}</span>
	{#if timeLabel}
		<span class="text-muted-foreground ml-auto shrink-0 tabular-nums">{timeLabel}</span>
	{/if}
</button>
