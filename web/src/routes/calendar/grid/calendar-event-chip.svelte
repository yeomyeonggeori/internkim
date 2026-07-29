<script lang="ts">
	import { cn } from '$lib/utils';
	import type { CalendarGridEvent } from './calendar-grid-layout';

	type CalendarEventChipSize = 'compact' | 'block';

	type CalendarEventChipProps = {
		event: CalendarGridEvent;
		size?: CalendarEventChipSize;
		timeLabel?: string;
		isSelected?: boolean;
		continuesBefore?: boolean;
		continuesAfter?: boolean;
		openEvent: (event: CalendarGridEvent, originElement: HTMLElement) => void;
		class?: string;
	};

	let {
		event,
		size = 'compact',
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
		'bg-(--calendar-event-color)/12 hover:bg-(--calendar-event-color)/22 text-foreground flex w-full min-w-0 overflow-hidden rounded-sm border-l-2 border-(--calendar-event-color) text-left text-xs leading-4 transition-colors',
		'focus-visible:ring-ring/50 outline-none focus-visible:ring-2',
		size === 'compact' && 'h-5 items-center gap-1.5 px-1.5',
		size === 'block' && 'h-full flex-col gap-0.5 px-1.5 py-1',
		continuesBefore && 'rounded-l-none border-l-0',
		continuesAfter && 'rounded-r-none',
		isSelected && 'ring-(--calendar-event-color) ring-2',
		className
	)}
	onclick={(clickEvent) => openEvent(event, clickEvent.currentTarget)}
>
	<span class="truncate font-medium">{event.title}</span>
	{#if timeLabel}
		<span class={cn('text-muted-foreground shrink-0 tabular-nums', size === 'compact' && 'ml-auto')}>{timeLabel}</span>
	{/if}
</button>
