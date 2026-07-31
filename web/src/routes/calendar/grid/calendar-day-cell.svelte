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
		isHoliday: boolean;
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
		isHoliday,
		addEventOnDay,
		selectDay,
		children
	}: CalendarDayCellProps = $props();

	const isRedDate = $derived(day.getDay() === 0 || day.getDay() === 6 || isHoliday);
</script>

<div
	role="gridcell"
	tabindex={isSelected ? 0 : -1}
	data-calendar-date={calendarGridDateKey(day)}
	data-selected={isSelected ? '' : undefined}
	class={cn(
		'border-border/50 flex h-24 min-w-0 flex-col gap-1 overflow-hidden border-r border-b px-1.5 pt-1 pb-1.5 last:border-r-0',
		'focus-visible:ring-ring/50 outline-none focus-visible:ring-2 focus-visible:ring-inset',
		isOutsideMonth && 'text-muted-foreground',
		isSelected && 'bg-accent/40'
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
			isOutsideMonth && !isToday && 'opacity-40',
			!isToday && isRedDate && 'text-destructive',
			isToday && !isRedDate && 'bg-primary text-primary-foreground',
			isToday && isRedDate && 'bg-destructive text-destructive-foreground'
		)}
	>
		{dayLabel}
	</span>
	<div class="flex min-h-0 min-w-0 flex-col gap-0.5">
		{@render children()}
	</div>
</div>
