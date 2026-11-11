<script lang="ts">
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import { Meter } from '$lib/components/ui/meter';
	import { cn } from '$lib/utils';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import ListTreeIcon from '@lucide/svelte/icons/list-tree';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TaskListBusinessCell from './task-list-business-cell.svelte';
	import { taskRoadmapDrag } from './task-roadmap-drag-action';
	import { taskRoadmapColumnsClass, taskRoadmapSpanStyle, taskRoadmapTodayLineLeft } from './task-roadmap-layout';
	import { formatTaskRoadmapPeriod, formatTaskRoadmapShortDate, type TaskRoadmapBar, type TaskRoadmapRow } from './task-roadmap-model';
	import type { TaskChildProgress } from './task-relationships';
	import { taskStatusIcon } from './task-status-style';
	import { statusIconClass } from './task-style';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskRoadmapText = PageText<typeof taskText>['task'];

	type Props = {
		row: TaskRoadmapRow;
		bar: TaskRoadmapBar;
		dayCount: number;
		todayIndex: number;
		redDayBackground: string;
		selected: boolean;
		dimmed: boolean;
		typeLabel: string;
		typeColor: string;
		businessColor: string;
		statusLabel: string;
		text: TaskRoadmapText;
		canUpdate: boolean;
		chooseBar: () => void;
		openTask: () => void;
		jumpTo: (dateISO: string) => void;
		previewMove: (dayOffset: number) => void;
		commitMove: (dayOffset: number) => void;
		cancelMove: () => void;
	};

	let { row, bar, dayCount, todayIndex, redDayBackground, selected, dimmed, typeLabel, typeColor, businessColor, statusLabel, text, canUpdate, chooseBar, openTask, jumpTo, previewMove, commitMove, cancelMove }: Props = $props();

	let laneWidth = $state(0);
	let StatusIcon = $derived(taskStatusIcon(row.task.status));
	let barColorStyle = $derived(`--roadmap-bar-color: ${typeColor}; color: color-mix(in oklch, var(--roadmap-bar-color) 45%, var(--foreground))`);
	let barFillStyle = $derived(`background: color-mix(in oklch, var(--roadmap-bar-color) 8%, var(--background)); border-color: color-mix(in oklch, var(--roadmap-bar-color) 45%, var(--border))`);

	function moveToLabel(dateLabel: string): string {
		return text.roadmap.moveToDate.replace('{date}', dateLabel);
	}

	function progressLabel(progress: TaskChildProgress): string {
		return text.relationships.progressLabel.replace('{completed}', String(progress.completed)).replace('{total}', String(progress.total));
	}

	function dayWidth(): number {
		return laneWidth / Math.max(1, dayCount);
	}

	function addDatesStyle(): string {
		return `left: ${taskRoadmapTodayLineLeft(Math.max(todayIndex, 0), dayCount)}`;
	}
</script>

<div class={cn(taskRoadmapColumnsClass, 'group border-b transition-opacity [contain-intrinsic-size:auto_3rem] [content-visibility:auto] max-sm:grid-rows-[auto_auto]', dimmed && 'opacity-40')} data-roadmap-task={row.task.id} data-roadmap-dimmed={dimmed ? 'true' : undefined}>
	<button type="button" class={cn('flex min-w-0 items-center gap-2.5 border-r px-3 py-2 text-left hover:bg-muted max-sm:col-span-3 max-sm:gap-2 max-sm:border-r-0 max-sm:pb-0 max-sm:hover:bg-transparent', selected && 'bg-muted max-sm:bg-transparent')} onclick={openTask} data-roadmap-title={row.task.id}>
		<span class="flex shrink-0" role="img" aria-label={statusLabel}><StatusIcon class={cn('size-3.5', statusIconClass(row.task.status))} aria-hidden="true" /></span>
		<PersonAvatar name={row.task.ownerName} seed={row.task.ownerID || row.task.ownerName} memberID={row.task.ownerID} class="size-6" />
		<span class="flex min-w-0 flex-1 flex-col gap-1.5">
			<span class="flex min-w-0 items-center gap-1.5">
				{#if row.childProgress}<ListTreeIcon class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" data-roadmap-parent-icon />{/if}
				<span class="truncate text-sm">{row.task.content}</span>
			</span>
			{#if row.childProgress}
				<span class="flex items-center gap-2" data-roadmap-progress>
					<Meter value={row.childProgress.percent} class="h-1 max-w-24" aria-label={progressLabel(row.childProgress)} />
					<span class="text-xs whitespace-nowrap text-muted-foreground">{progressLabel(row.childProgress)}</span>
				</span>
			{/if}
		</span>
		{#if row.task.business}<span class="flex shrink-0 sm:max-lg:hidden"><TaskListBusinessCell label={row.task.business} color={businessColor} /></span>{/if}
		<span class="flex shrink-0 max-sm:hidden"><DefinitionBadge label={typeLabel} color={typeColor} /></span>
	</button>
	<div></div>
	<div class="relative h-12 overflow-hidden max-sm:h-9 sm:[background-image:var(--roadmap-red-days)]" style={`--roadmap-red-days: ${redDayBackground}`} bind:clientWidth={laneWidth}>
		{#if todayIndex >= 0}
			<div class="pointer-events-none absolute inset-y-0 w-px bg-primary/25 max-sm:hidden" style={`left: ${taskRoadmapTodayLineLeft(todayIndex, dayCount)}`} data-roadmap-today-line></div>
		{/if}
		{#if bar.kind === 'span'}
			<div class="absolute inset-y-0 z-[1] my-auto h-7" style={`${taskRoadmapSpanStyle(bar, dayCount)}; ${barColorStyle}`} data-roadmap-slide="move">
				<button
					type="button"
					class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'absolute inset-0 h-full min-h-0 w-full min-w-0 active:not-aria-[haspopup]:translate-y-0', canUpdate && 'cursor-grab active:cursor-grabbing', selected && 'ring-2 ring-[var(--roadmap-bar-color)]')}
					style={barFillStyle}
					title={row.task.content}
					aria-label={`${row.task.content}, ${formatTaskRoadmapPeriod(bar)}`}
					aria-pressed={selected}
					onclick={chooseBar}
					data-roadmap-bar={row.task.id}
					use:taskRoadmapDrag={{ enabled: canUpdate, dayWidth, onPreview: previewMove, onCommit: commitMove, onCancel: cancelMove }}
				></button>
				<div class="pointer-events-none relative flex h-full min-w-0 items-center gap-1 px-2.5 text-[0.8rem] font-medium tabular-nums">
					{#if bar.startsBefore}
						<button type="button" class="pointer-events-auto flex shrink-0 items-center gap-1 rounded-sm hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={moveToLabel(formatTaskRoadmapShortDate(bar.startDate))} onclick={() => jumpTo(bar.startDate)} data-roadmap-jump="start">
							<ArrowLeftIcon class="size-3.5" aria-hidden="true" />{formatTaskRoadmapShortDate(bar.startDate)}
						</button>
					{/if}
					<span class="min-w-0 truncate font-semibold sm:hidden">{typeLabel}</span>
					{#if bar.endsAfter}
						<button type="button" class="pointer-events-auto ml-auto flex shrink-0 items-center gap-1 rounded-sm hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={moveToLabel(formatTaskRoadmapShortDate(bar.endDate))} onclick={() => jumpTo(bar.endDate)} data-roadmap-jump="end">
							{formatTaskRoadmapShortDate(bar.endDate)}<ArrowRightIcon class="size-3.5" aria-hidden="true" />
						</button>
					{/if}
				</div>
			</div>
		{:else if bar.kind === 'before'}
			<Button variant="ghost" size="sm" class="absolute inset-y-0 left-1.5 my-auto text-muted-foreground tabular-nums" aria-label={moveToLabel(formatTaskRoadmapPeriod(bar))} onclick={() => jumpTo(bar.startDate)} data-roadmap-slide="fade" data-roadmap-away={row.task.id}>
				<ArrowLeftIcon />{formatTaskRoadmapPeriod(bar)}
			</Button>
		{:else if bar.kind === 'after'}
			<Button variant="ghost" size="sm" class="absolute inset-y-0 right-1.5 my-auto text-muted-foreground tabular-nums" aria-label={moveToLabel(formatTaskRoadmapPeriod(bar))} onclick={() => jumpTo(bar.startDate)} data-roadmap-slide="fade" data-roadmap-away={row.task.id}>
				{formatTaskRoadmapPeriod(bar)}<ArrowRightIcon />
			</Button>
		{:else}
			<Button variant="ghost" size="sm" class="absolute inset-y-0 my-auto border-dashed border-border text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100" style={addDatesStyle()} onclick={openTask} data-roadmap-add-dates={row.task.id}>
				<PlusIcon />{text.roadmap.addDates}
			</Button>
		{/if}
	</div>
	<div></div>
</div>
