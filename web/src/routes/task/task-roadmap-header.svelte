<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import { cn } from '$lib/utils';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { TaskRoadmapHolidays } from './task-roadmap-holidays';
	import { taskRoadmapColumnsClass, taskRoadmapSpanStyle, type TaskRoadmapDaySpan } from './task-roadmap-layout';
	import { formatTaskRoadmapDayNumber, formatTaskRoadmapShortDate, formatTaskRoadmapWeekday } from './task-roadmap-model';
	import type { TaskRoadmapSwipeDirection } from './task-roadmap-swipe-action';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskRoadmapText = PageText<typeof taskText>['task']['roadmap'];

	type Props = {
		days: string[];
		todayISO: string;
		redDays: boolean[];
		holidays: TaskRoadmapHolidays;
		selectedSpan: TaskRoadmapDaySpan | undefined;
		selectedColor: string;
		text: TaskRoadmapText;
		canMoveBack: boolean;
		canMoveForward: boolean;
		moveWeek: (direction: TaskRoadmapSwipeDirection) => void;
		chooseDay: (dayISO: string) => void;
		laneWidth: number;
	};

	let { days, todayISO, redDays, holidays, selectedSpan, selectedColor, text, canMoveBack, canMoveForward, moveWeek, chooseDay, laneWidth = $bindable() }: Props = $props();

	function showFromLabel(dayISO: string): string {
		return text.showFromDay.replace('{date}', formatTaskRoadmapShortDate(dayISO));
	}
</script>

<div class={cn(taskRoadmapColumnsClass, 'sticky top-0 z-20 overflow-x-clip border-b bg-background')}>
	<div class="flex items-center border-r px-3 text-xs font-medium text-muted-foreground max-sm:hidden">{text.taskColumn}</div>
	<div class="grid place-items-center">
		<TooltipIconButton label={text.previousWeek} variant="secondary" size="icon-sm" disabled={!canMoveBack} onclick={() => moveWeek(-1)} data-roadmap-move="previous">
			<ChevronLeftIcon />
		</TooltipIconButton>
	</div>
	<div class="relative grid overflow-hidden py-1.5" style={`grid-template-columns: repeat(${days.length}, minmax(0, 1fr))`} bind:clientWidth={laneWidth}>
		{#if selectedSpan}
			<div class="pointer-events-none absolute bottom-2 h-5 rounded-full" style={`${taskRoadmapSpanStyle(selectedSpan, days.length)}; background: color-mix(in oklch, ${selectedColor} 18%, var(--background))`} data-roadmap-slide="move" data-roadmap-selected-span></div>
		{/if}
		{#each days as dayISO, index (dayISO)}
			{@const red = redDays[index] ?? false}
			{@const today = dayISO === todayISO}
			<button
				type="button"
				class={cn('relative z-[1] flex h-9 min-w-0 flex-col items-center justify-center rounded-md text-xs leading-none tabular-nums hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none', red ? 'text-destructive' : 'text-muted-foreground')}
				title={holidays.titles.get(dayISO)}
				aria-label={showFromLabel(dayISO)}
				onclick={() => chooseDay(dayISO)}
				data-roadmap-slide="move"
				data-roadmap-day={dayISO}
				data-roadmap-red-day={red ? 'true' : undefined}
			>
				<span class="text-[0.625rem]">{formatTaskRoadmapWeekday(dayISO, text.dateLocale)}</span>
				<span class={cn('mt-0.5 inline-flex h-5 min-w-5 items-center justify-center rounded-full px-0.5 whitespace-nowrap', today && 'bg-primary font-semibold text-primary-foreground')} data-roadmap-today={today ? 'true' : undefined}>{formatTaskRoadmapDayNumber(dayISO)}</span>
			</button>
		{/each}
	</div>
	<div class="grid place-items-center">
		<TooltipIconButton label={text.nextWeek} variant="secondary" size="icon-sm" disabled={!canMoveForward} onclick={() => moveWeek(1)} data-roadmap-move="next">
			<ChevronRightIcon />
		</TooltipIconButton>
	</div>
</div>
