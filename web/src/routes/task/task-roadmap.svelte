<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { Meter } from '$lib/components/ui/meter';
	import { cn } from '$lib/utils';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { taskRoadmapDrag, type TaskRoadmapDragOffsets } from './task-roadmap-drag-action';
	import { hasRelatedTasks, relatedTaskIDs } from './task-roadmap-focus';
	import {
		buildTaskRoadmap,
		shiftTaskDateRange,
		taskDateRange,
		taskRoadmapBar,
		taskRoadmapMonthDays,
		taskRoadmapMonthOf,
		type TaskDateRange,
		type TaskRoadmapBar,
		type TaskRoadmapMonth
	} from './task-roadmap-model';
	import { buildTaskChildProgressByParent, type TaskChildProgress } from './task-relationships';
	import { isTaskStatusCompleted, isTaskStatusInProgress } from './task-status';
	import type { Task } from './task-types';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskRoadmapText = PageText<typeof taskText>['task'];

	type Props = {
		tasks: Task[];
		allTasks: Task[];
		serverChildProgress?: Record<string, TaskChildProgress>;
		month: TaskRoadmapMonth;
		todayISO: string;
		text: TaskRoadmapText;
		statusLabel: (status: string) => string;
		openTask: (task: Task) => void;
		canUpdateTask: (task: Task) => boolean;
		updateTaskDates: (task: Task, startDate: string, endDate: string) => Promise<boolean>;
		selectMonth: (month: TaskRoadmapMonth) => void;
	};

	let { tasks, allTasks, serverChildProgress, month, todayISO, text, statusLabel, openTask, canUpdateTask, updateTaskDates, selectMonth }: Props = $props();

	const dayWidthRem = 2;
	let childProgressByParent = $derived(serverChildProgress ? new Map(Object.entries(serverChildProgress)) : buildTaskChildProgressByParent(allTasks));
	let days = $derived(taskRoadmapMonthDays(month));
	let todayIndex = $derived(days.indexOf(todayISO));
	let groups = $derived(buildTaskRoadmap(tasks, month, childProgressByParent));
	let collapsedStatuses = $state(new Set(['completed']));
	let selectedTaskID = $state('');
	let highlightedTaskIDs = $derived(selectedTaskID ? relatedTaskIDs(selectedTaskID, allTasks) : null);
	let movedRanges = $state(new Map<string, TaskDateRange>());
	let timelineElement = $state<HTMLElement | null>(null);
	let roadmapElement = $state<HTMLElement | null>(null);

	function dayWidthPixels(): number {
		return timelineElement ? timelineElement.clientWidth / Math.max(1, days.length) : 32;
	}

	function shownBar(task: Task, bar: TaskRoadmapBar): TaskRoadmapBar {
		const moved = movedRanges.get(task.id);
		return moved ? taskRoadmapBar({ ...task, ...moved }, days) : bar;
	}

	function chooseTask(task: Task): void {
		if (selectedTaskID === task.id || !hasRelatedTasks(task.id, allTasks)) {
			openTask(task);
			return;
		}
		selectedTaskID = task.id;
	}

	function clearSelectionOutsideTasks(event: MouseEvent): void {
		if (!selectedTaskID || !(event.target instanceof Element) || !roadmapElement?.contains(event.target)) return;
		if (!event.target.closest('button')) selectedTaskID = '';
	}

	function toggleGroup(status: string, open: boolean): void {
		const next = new Set(collapsedStatuses);
		if (open) next.delete(status);
		else next.add(status);
		collapsedStatuses = next;
	}

	function setMovedRange(taskID: string, range: TaskDateRange | undefined): void {
		const next = new Map(movedRanges);
		if (range) next.set(taskID, range);
		else next.delete(taskID);
		movedRanges = next;
	}

	function previewDrag(task: Task, offsets: TaskRoadmapDragOffsets): void {
		const range = taskDateRange(task);
		if (range) setMovedRange(task.id, shiftTaskDateRange(range, offsets.startOffset, offsets.endOffset));
	}

	async function commitDrag(task: Task, offsets: TaskRoadmapDragOffsets): Promise<void> {
		const range = taskDateRange(task);
		if (!range) return;
		const next = shiftTaskDateRange(range, offsets.startOffset, offsets.endOffset);
		setMovedRange(task.id, next);
		await updateTaskDates(task, next.startDate, next.endDate);
		setMovedRange(task.id, undefined);
	}

	function barVariant(status: string): 'default' | 'outline' | 'secondary' {
		if (isTaskStatusInProgress(status)) return 'default';
		if (isTaskStatusCompleted(status)) return 'secondary';
		return 'outline';
	}

	function shortDate(dateISO: string): string {
		return new Intl.DateTimeFormat(text.roadmap.dateLocale, { month: 'numeric', day: 'numeric', timeZone: 'UTC' }).format(new Date(`${dateISO}T00:00:00Z`));
	}

	function dateRangeLabel(task: Task): string {
		const range = movedRanges.get(task.id) ?? taskDateRange(task);
		if (!range) return '';
		return range.startDate === range.endDate ? shortDate(range.startDate) : `${shortDate(range.startDate)} – ${shortDate(range.endDate)}`;
	}

	function progressLabel(progress: TaskChildProgress): string {
		return text.relationships.progressLabel.replace('{completed}', String(progress.completed)).replace('{total}', String(progress.total));
	}

	function laneStyle(): string {
		return `width: ${days.length * dayWidthRem}rem`;
	}

	function spanStyle(bar: Extract<TaskRoadmapBar, { kind: 'span' }>): string {
		return `left: ${bar.firstDay * dayWidthRem}rem; width: ${(bar.lastDay - bar.firstDay + 1) * dayWidthRem}rem`;
	}
</script>

<svelte:window onclick={clearSelectionOutsideTasks} />

<div bind:this={roadmapElement} class="overflow-x-auto rounded-lg border" data-task-roadmap>
	<div class="w-max min-w-full">
		<div class="flex border-b">
			<div class="sticky left-0 z-20 flex w-80 shrink-0 items-end border-r bg-background px-3 pb-2 text-xs font-medium text-muted-foreground max-sm:w-56">{text.roadmap.taskColumn}</div>
			<div bind:this={timelineElement} class="grid shrink-0" style={`${laneStyle()}; grid-template-columns: repeat(${days.length}, minmax(0, 1fr))`}>
				{#each days as dayISO, index (dayISO)}
					<div class="flex h-14 items-end justify-center pb-2 text-xs tabular-nums text-muted-foreground">
						<span class={cn('inline-flex size-6 items-center justify-center rounded-full', index === todayIndex && 'bg-primary font-semibold text-primary-foreground')} data-roadmap-today={index === todayIndex ? 'true' : undefined}>{index + 1}</span>
					</div>
				{/each}
			</div>
		</div>

		{#each groups as group (group.status)}
			{@const open = !collapsedStatuses.has(group.status)}
			<Collapsible.Root {open} onOpenChange={(next) => toggleGroup(group.status, next)}>
				<div class="flex border-b bg-muted/60">
					<div class="sticky left-0 z-20 flex h-10 w-80 shrink-0 items-center bg-muted px-2 max-sm:w-56">
						<Collapsible.Trigger class={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'gap-2')} data-roadmap-group={group.status}>
							<ChevronRightIcon class={cn('transition-transform', open && 'rotate-90')} />
							<span class="font-semibold">{statusLabel(group.status)}</span>
							<Badge variant="secondary" class="bg-background">{group.rows.length}</Badge>
						</Collapsible.Trigger>
					</div>
					<div class="shrink-0" style={laneStyle()}></div>
				</div>
				<Collapsible.Content>
					{#each group.rows as row (row.task.id)}
						{@const bar = shownBar(row.task, row.bar)}
						{@const dimmed = highlightedTaskIDs !== null && !highlightedTaskIDs.has(row.task.id)}
						{@const selected = selectedTaskID === row.task.id}
						<div class={cn('flex border-b transition-opacity', dimmed && 'opacity-40', selected && 'bg-muted/50')} data-roadmap-task={row.task.id} data-roadmap-dimmed={dimmed ? 'true' : undefined}>
							<button type="button" class={cn('sticky left-0 z-10 flex w-80 shrink-0 items-center gap-2.5 border-r bg-background py-2 pr-3 pl-9 text-left max-sm:w-56 max-sm:pl-4', selected && 'bg-muted')} onclick={() => chooseTask(row.task)}>
								<PersonAvatar name={row.task.ownerName} seed={row.task.ownerID || row.task.ownerName} memberID={row.task.ownerID} class="size-6" />
								<span class="flex min-w-0 flex-1 flex-col gap-1.5">
									<span class="truncate text-sm">{row.task.content}</span>
									{#if selected}
										<span class="text-xs text-muted-foreground" data-roadmap-open-hint>{text.roadmap.openAgain}</span>
									{/if}
									{#if row.childProgress}
										<span class="flex items-center gap-2" data-roadmap-progress>
											<Meter value={row.childProgress.percent} class="h-1 max-w-24" aria-label={progressLabel(row.childProgress)} />
											<span class="text-xs whitespace-nowrap text-muted-foreground">{progressLabel(row.childProgress)}</span>
										</span>
									{/if}
								</span>
								{#if row.task.business}<Badge variant="outline">{row.task.business}</Badge>{/if}
							</button>
							<div class="relative shrink-0" style={laneStyle()}>
								{#if todayIndex >= 0}
									<div class="pointer-events-none absolute inset-y-0 z-0 w-0.5 -translate-x-1/2 bg-primary" style={`left: ${(todayIndex + 0.5) * dayWidthRem}rem`}></div>
								{/if}
								{#if bar.kind === 'span'}
									<button
										type="button"
										class={cn(buttonVariants({ variant: barVariant(row.task.status), size: 'sm' }), 'absolute top-1/2 z-[1] -translate-y-1/2 justify-start overflow-hidden', canUpdateTask(row.task) ? 'cursor-grab active:cursor-grabbing' : '', selected && 'ring-2 ring-ring', isTaskStatusCompleted(row.task.status) && 'line-through')}
										style={spanStyle(bar)}
										data-roadmap-bar={row.task.id}
										onclick={() => chooseTask(row.task)}
										use:taskRoadmapDrag={{
											enabled: canUpdateTask(row.task),
											dayWidth: dayWidthPixels,
											onPreview: (offsets) => previewDrag(row.task, offsets),
											onCommit: (offsets) => void commitDrag(row.task, offsets),
											onCancel: () => setMovedRange(row.task.id, undefined)
										}}
									>
										{#if canUpdateTask(row.task) && !bar.startsBefore}<span class="absolute inset-y-1 left-0.5 w-1.5 cursor-ew-resize rounded-full opacity-0 hover:opacity-60 bg-current" data-roadmap-handle="start" title={text.roadmap.resizeStart}></span>{/if}
										<span class="truncate">{row.task.content}</span>
										<span class="truncate font-normal opacity-70">{dateRangeLabel(row.task)}</span>
										{#if canUpdateTask(row.task) && !bar.endsAfter}<span class="absolute inset-y-1 right-0.5 w-1.5 cursor-ew-resize rounded-full opacity-0 hover:opacity-60 bg-current" data-roadmap-handle="end" title={text.roadmap.resizeEnd}></span>{/if}
									</button>
								{:else if bar.kind === 'before'}
									<button type="button" class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'absolute top-1/2 left-2 -translate-y-1/2')} onclick={() => selectMonth(taskRoadmapMonthOf(bar.dateISO))}>
										<ArrowLeftIcon />{row.task.content} · {shortDate(bar.dateISO)}
									</button>
								{:else if bar.kind === 'after'}
									<button type="button" class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'absolute top-1/2 right-2 -translate-y-1/2')} onclick={() => selectMonth(taskRoadmapMonthOf(bar.dateISO))}>
										{row.task.content} · {shortDate(bar.dateISO)}<ArrowRightIcon />
									</button>
								{:else}
									<button type="button" class={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'absolute top-1/2 -translate-y-1/2 border-dashed border-border text-muted-foreground')} style={`left: ${(Math.max(todayIndex, 0) + 1) * dayWidthRem}rem`} onclick={() => openTask(row.task)}>
										<PlusIcon />{text.roadmap.addDates}
									</button>
								{/if}
								<div class="h-12"></div>
							</div>
						</div>
					{/each}
				</Collapsible.Content>
			</Collapsible.Root>
		{/each}
	</div>
</div>
