<script lang="ts">
	import { cn } from '$lib/utils';
	import type { CalendarGridEvent } from './calendar-grid-layout';

	type CalendarEventChipSize = 'compact' | 'block';

	type CalendarEventChipProps = {
		event: CalendarGridEvent;
		size?: CalendarEventChipSize;
		timeLabel?: string;
		placeholder?: string;
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
		placeholder = '',
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
	aria-disabled={event.readOnly ? 'true' : undefined}
	tabindex={event.readOnly ? -1 : undefined}
	style={`--calendar-event-color: ${event.color}`}
	class={cn(
		'flex w-full min-w-0 gap-1.5 overflow-hidden rounded-md px-1 text-left text-xs leading-4 transition-colors',
		'focus-visible:ring-ring/50 outline-none focus-visible:ring-2',
		isSelected
			? 'bg-(--calendar-event-color) text-white'
			: 'bg-(--calendar-event-color)/12 hover:bg-(--calendar-event-color)/22 text-foreground',
		size === 'compact' && 'h-5 items-center py-0.5',
		size === 'block' && 'h-full items-stretch py-1',
		continuesBefore && 'rounded-l-none',
		continuesAfter && 'rounded-r-none',
		className
	)}
	onclick={(clickEvent) => {
		if (event.readOnly) return;
		openEvent(event, clickEvent.currentTarget);
	}}
	oncontextmenu={(contextMenuEvent) => {
		if (!event.readOnly) return;
		contextMenuEvent.preventDefault();
		contextMenuEvent.stopPropagation();
	}}
>
	{#if !continuesBefore}
		<span
			class={cn(
				'calendar-event-accent w-1 shrink-0 self-stretch rounded-full bg-(--calendar-event-color)',
				isSelected && 'invisible'
			)}
		></span>
	{/if}
	<span class={cn('flex min-w-0 flex-1 gap-1.5', size === 'compact' ? 'items-center' : 'flex-col')}>
		<span class={cn('truncate font-medium', !event.title && 'opacity-70')}>{event.title || placeholder}</span>
		{#if timeLabel}
			<span class={cn('shrink-0 tabular-nums', isSelected ? 'text-white/80' : 'text-muted-foreground', size === 'compact' && 'ml-auto')}>
			{timeLabel}
		</span>
		{/if}
	</span>
</button>
