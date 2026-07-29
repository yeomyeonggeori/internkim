<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import { cn } from '$lib/utils';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import TrashIcon from '@lucide/svelte/icons/trash';
	import type { CalendarGridEvent } from './calendar-grid-layout';

	type CalendarEventChipProps = {
		event: CalendarGridEvent;
		variant?: 'timed' | 'span';
		timeLabel?: string;
		isSelected?: boolean;
		continuesBefore?: boolean;
		continuesAfter?: boolean;
		text: {
			editEvent: string;
			duplicateEvent: string;
			deleteEvent: string;
		};
		openEvent: (event: CalendarGridEvent) => void;
		duplicateEvent: (event: CalendarGridEvent) => void;
		deleteEvent: (event: CalendarGridEvent) => void;
		class?: string;
	};

	let {
		event,
		variant = 'timed',
		timeLabel = '',
		isSelected = false,
		continuesBefore = false,
		continuesAfter = false,
		text,
		openEvent,
		duplicateEvent,
		deleteEvent,
		class: className
	}: CalendarEventChipProps = $props();
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				data-calendar-event-id={event.id}
				data-selected={isSelected ? '' : undefined}
				style={`--calendar-event-color: ${event.color}`}
				class={cn(
					'group/event flex w-full min-w-0 items-center gap-1.5 overflow-hidden text-left text-xs leading-4 transition-colors',
					'focus-visible:ring-ring/50 outline-none focus-visible:ring-2',
					variant === 'timed' && 'rounded-sm px-1.5 py-0.5 hover:bg-(--calendar-event-color)/15',
					variant === 'span' &&
						'bg-(--calendar-event-color)/15 text-foreground h-5 rounded-sm px-1.5 font-medium hover:bg-(--calendar-event-color)/25',
					continuesBefore && 'rounded-l-none',
					continuesAfter && 'rounded-r-none',
					isSelected && 'ring-(--calendar-event-color) ring-2',
					className
				)}
				onclick={() => openEvent(event)}
			>
				{#if variant === 'timed'}
					<span class="size-1.5 shrink-0 rounded-full bg-(--calendar-event-color)"></span>
				{/if}
				<span class="truncate font-medium">{event.title}</span>
				{#if timeLabel}
					<span class="text-muted-foreground ml-auto shrink-0 tabular-nums">{timeLabel}</span>
				{/if}
			</button>
		{/snippet}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-44">
		<ContextMenu.Item onclick={() => openEvent(event)}>
			<PencilIcon />
			{text.editEvent}
		</ContextMenu.Item>
		<ContextMenu.Item onclick={() => duplicateEvent(event)}>
			<CopyIcon />
			{text.duplicateEvent}
		</ContextMenu.Item>
		<ContextMenu.Separator />
		<ContextMenu.Item variant="destructive" onclick={() => deleteEvent(event)}>
			<TrashIcon />
			{text.deleteEvent}
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
