<script lang="ts">
	import { cn } from '$lib/utils';
	import type { Snippet } from 'svelte';
	import { calendarGridDateKey } from './calendar-grid-dates';

	type CalendarDayCellProps = {
		day: Date;
		dayLabel: string;
		isToday: boolean;
		isOutsideMonth: boolean;
		isSelected: boolean;
		isInSelectedRange?: boolean;
		addEventOnDay: (day: Date) => void;
		selectDay: (day: Date) => void;
		children: Snippet;
	};

	let {
		day,
		dayLabel,
		isToday,
		isOutsideMonth,
		isSelected,
		isInSelectedRange = false,
		addEventOnDay,
		selectDay,
		children
	}: CalendarDayCellProps = $props();
</script>

<div
	role="gridcell"
	tabindex={isSelected ? 0 : -1}
	data-calendar-date={calendarGridDateKey(day)}
	data-selected={isSelected ? '' : undefined}
	class={cn(
		'border-border/70 flex min-h-24 min-w-0 flex-col gap-1 border-r border-b px-1.5 pt-1 pb-1.5 last:border-r-0',
		'focus-visible:ring-ring/50 outline-none focus-visible:ring-2 focus-visible:ring-inset',
		isOutsideMonth && 'text-muted-foreground',
		isSelected && 'bg-accent/40',
		isInSelectedRange && 'bg-primary/10'
	)}
	onclick={() => selectDay(day)}
	ondblclick={() => addEventOnDay(day)}
	onkeydown={(keyboardEvent) => {
		if (keyboardEvent.key !== 'Enter' && keyboardEvent.key !== ' ') return;
		keyboardEvent.preventDefault();
		addEventOnDay(day);
	}}
>
	<span
		class={cn(
			'flex size-6 shrink-0 items-center justify-center self-start rounded-full text-xs font-medium tabular-nums',
			isToday && 'bg-primary text-primary-foreground'
		)}
	>
		{dayLabel}
	</span>
	<div class="flex min-h-0 min-w-0 flex-col gap-0.5">
		{@render children()}
	</div>
</div>
