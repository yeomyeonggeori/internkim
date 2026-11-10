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
	import ListTreeIcon from '@lucide/svelte/icons/list-tree';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { onMount, untrack } from 'svelte';
	import { taskRoadmapDrag } from './task-roadmap-drag-action';
	import { hasRelatedTasks, relatedTaskIDs } from './task-roadmap-focus';
	import { loadTaskRoadmapHolidays, noTaskRoadmapHolidays } from './task-roadmap-holidays';
	import {
		buildTaskRoadmap,
		formatTaskRoadmapMonth,
		formatTaskRoadmapWeekday,
		isTaskRoadmapWeekend,
		moveTaskDateRange,
		taskDateRange,
		taskRoadmapBar,
		taskRoadmapDays,
		taskRoadmapMonthKey,
		taskRoadmapMonthSegments,
		type TaskDateRange,
		type TaskRoadmapBar,
		type TaskRoadmapMonth
	} from './task-roadmap-model';
	import { taskRoadmapFocusScrollDay, taskRoadmapMonthScrollDay, taskRoadmapTodayScrollDay, taskRoadmapVisibleMonth } from './task-roadmap-scroll';
	import TaskListBusinessCell from './task-list-business-cell.svelte';
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
		centerMonth: TaskRoadmapMonth;
		month: TaskRoadmapMonth;
		todayISO: string;
		text: TaskRoadmapText;
		statusLabel: (status: string) => string;
		businessColor: (business: string | null) => string;
		openTask: (task: Task) => void;
		canUpdateTask: (task: Task) => boolean;
		updateTaskDates: (task: Task, startDate: string, endDate: string) => Promise<boolean>;
		selectMonth: (month: TaskRoadmapMonth) => void;
	};

	let { tasks, allTasks, serverChildProgress, centerMonth, month, todayISO, text, statusLabel, businessColor, openTask, canUpdateTask, updateTaskDates, selectMonth }: Props = $props();

	const dayWidthRem = 2;
	const leftColumnClass = 'w-80 max-sm:w-56';
	let childProgressByParent = $derived(serverChildProgress ? new Map(Object.entries(serverChildProgress)) : buildTaskChildProgressByParent(allTasks));
	let days = $derived(taskRoadmapDays(centerMonth));
	let monthSegments = $derived(taskRoadmapMonthSegments(days));
	let todayIndex = $derived(days.indexOf(todayISO));
	let groups = $derived(buildTaskRoadmap(tasks, days, childProgressByParent));
	let holidays = $state(noTaskRoadmapHolidays);
	let collapsedStatuses = $state(new Set<string>());
	let selectedTaskID = $state('');
	let highlightedTaskIDs = $derived(selectedTaskID ? relatedTaskIDs(selectedTaskID, allTasks) : null);
	let movedRanges = $state(new Map<string, TaskDateRange>());
	let scrollerElement = $state<HTMLElement | null>(null);
	let timelineElement = $state<HTMLElement | null>(null);
	let leftColumnElement = $state<HTMLElement | null>(null);

	onMount(() => {
		scrollToToday();
		const firstDayISO = days[0];
		const lastDayISO = days.at(-1);
		if (!firstDayISO || !lastDayISO) return;
		loadTaskRoadmapHolidays(firstDayISO, lastDayISO, text.roadmap.holidayLocale === 'en' ? 'en' : 'ko')
			.then((loaded) => { holidays = loaded; })
			.catch((error: unknown) => { console.warn('task roadmap holidays unavailable', { error }); });
	});

	$effect(() => {
		const selectedKey = taskRoadmapMonthKey(month);
		untrack(() => {
			const visible = visibleMonth();
			if (!visible || selectedKey !== taskRoadmapMonthKey(visible)) scrollToMonth(month);
		});
	});

	function dayWidthPixels(): number {
		return timelineElement ? timelineElement.clientWidth / Math.max(1, days.length) : 32;
	}

	function timelineViewportWidth(): number {
		return (scrollerElement?.clientWidth ?? 0) - (leftColumnElement?.offsetWidth ?? 0);
	}

	function scrollToDay(dayIndex: number): void {
		if (!scrollerElement) return;
		scrollerElement.scrollLeft = Math.max(0, dayIndex) * dayWidthPixels();
	}

	function visibleDays(): number {
		return timelineViewportWidth() / dayWidthPixels();
	}

	function scrollToToday(): void {
		scrollToDay(taskRoadmapTodayScrollDay(monthSegments, todayIndex, visibleDays()));
	}

	function focusDay(dayIndex: number): void {
		scrollToDay(taskRoadmapFocusScrollDay(dayIndex, visibleDays()));
	}

	function focusBar(bar: TaskRoadmapBar): void {
		if (bar.kind === 'span') focusDay(bar.firstDay);
		else if (bar.kind === 'before') scrollToDay(0);
		else if (bar.kind === 'after') scrollToDay(days.length);
	}

	function chooseTaskFromList(task: Task, bar: TaskRoadmapBar): void {
		focusBar(bar);
		chooseTask(task);
	}

	function scrollToMonth(target: TaskRoadmapMonth): void {
		const firstDay = taskRoadmapMonthScrollDay(monthSegments, target);
		if (firstDay !== undefined) scrollToDay(firstDay);
	}

	function visibleMonth(): TaskRoadmapMonth | undefined {
		if (!scrollerElement) return undefined;
		return taskRoadmapVisibleMonth(days, scrollerElement.scrollLeft / dayWidthPixels(), visibleDays());
	}

	function followScroll(): void {
		const visible = visibleMonth();
		if (visible && taskRoadmapMonthKey(visible) !== taskRoadmapMonthKey(month)) selectMonth(visible);
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
		if (!selectedTaskID || !(event.target instanceof Element) || !scrollerElement?.contains(event.target)) return;
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

	function previewMove(task: Task, dayOffset: number): void {
		const range = taskDateRange(task);
		if (range) setMovedRange(task.id, moveTaskDateRange(range, dayOffset));
	}

	async function commitMove(task: Task, dayOffset: number): Promise<void> {
		const range = taskDateRange(task);
		if (!range) return;
		const next = moveTaskDateRange(range, dayOffset);
		setMovedRange(task.id, next);
		await updateTaskDates(task, next.startDate, next.endDate);
		setMovedRange(task.id, undefined);
	}

	function barVariant(status: string): 'default' | 'outline' | 'secondary' {
		if (isTaskStatusInProgress(status)) return 'default';
		if (isTaskStatusCompleted(status)) return 'secondary';
		return 'outline';
	}

	function isRedDay(dayISO: string): boolean {
		return isTaskRoadmapWeekend(dayISO) || holidays.dates.has(dayISO);
	}

	function shortDate(dateISO: string): string {
		return new Intl.DateTimeFormat(text.roadmap.dateLocale, { month: 'numeric', day: 'numeric', timeZone: 'UTC' }).format(new Date(`${dateISO}T00:00:00Z`));
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

<div bind:this={scrollerElement} class="overflow-x-auto overscroll-x-contain rounded-lg border" onscroll={followScroll} data-task-roadmap>
	<div class="w-max min-w-full">
		<div class="flex border-b">
			<div bind:this={leftColumnElement} class={cn('sticky left-0 z-20 flex shrink-0 items-end border-r bg-background px-3 pb-2 text-xs font-medium text-muted-foreground', leftColumnClass)}>{text.roadmap.taskColumn}</div>
			<div bind:this={timelineElement} class="shrink-0" style={laneStyle()}>
				<div class="flex h-5">
					{#each monthSegments as segment (taskRoadmapMonthKey(segment.month))}
						<div class="border-l px-2 text-xs leading-5 font-semibold whitespace-nowrap first:border-l-0" style={`width: ${segment.dayCount * dayWidthRem}rem`} data-roadmap-month={taskRoadmapMonthKey(segment.month)}>
							<span class="sticky left-82 max-sm:left-58">{formatTaskRoadmapMonth(segment.month, text.roadmap.dateLocale)}</span>
						</div>
					{/each}
				</div>
				<div class="grid h-9" style={`grid-template-columns: repeat(${days.length}, minmax(0, 1fr))`}>
					{#each days as dayISO, index (dayISO)}
						{@const red = isRedDay(dayISO)}
						<button type="button" class={cn('flex flex-col items-center justify-center rounded-md text-xs leading-none tabular-nums hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none', red ? 'text-destructive' : 'text-muted-foreground')} title={holidays.titles.get(dayISO)} onclick={() => focusDay(index)} data-roadmap-day={dayISO} data-roadmap-red-day={red ? 'true' : undefined}>
							<span class="text-[0.625rem]">{formatTaskRoadmapWeekday(dayISO, text.roadmap.dateLocale)}</span>
							<span class={cn('mt-0.5 inline-flex size-5 items-center justify-center rounded-full', index === todayIndex && 'bg-primary font-semibold text-primary-foreground')} data-roadmap-today={index === todayIndex ? 'true' : undefined}>{Number(dayISO.slice(8))}</span>
						</button>
					{/each}
				</div>
			</div>
		</div>

		{#each groups as group (group.status)}
			{@const open = !collapsedStatuses.has(group.status)}
			<Collapsible.Root {open} onOpenChange={(next) => toggleGroup(group.status, next)}>
				<div class="flex border-b bg-muted/60">
					<div class={cn('sticky left-0 z-20 flex h-10 shrink-0 items-center bg-muted px-2', leftColumnClass)}>
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
						<div class={cn('group flex border-b transition-opacity', dimmed && 'opacity-40', selected && 'bg-muted/50')} data-roadmap-task={row.task.id} data-roadmap-dimmed={dimmed ? 'true' : undefined}>
							<button type="button" class={cn('sticky left-0 z-10 flex shrink-0 items-center gap-2.5 border-r bg-background py-2 pr-3 pl-9 text-left max-sm:pl-4', leftColumnClass, selected && 'bg-muted')} onclick={() => chooseTaskFromList(row.task, bar)}>
								<PersonAvatar name={row.task.ownerName} seed={row.task.ownerID || row.task.ownerName} memberID={row.task.ownerID} class="size-6" />
								<span class="flex min-w-0 flex-1 flex-col gap-1.5">
									<span class="flex min-w-0 items-center gap-1.5">
										{#if row.childProgress}<ListTreeIcon class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" data-roadmap-parent-icon />{/if}
										<span class="truncate text-sm">{row.task.content}</span>
									</span>
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
								{#if row.task.business}<TaskListBusinessCell label={row.task.business} color={businessColor(row.task.business)} />{/if}
							</button>
							<div class="relative shrink-0" style={laneStyle()}>
								{#if todayIndex >= 0}
									<div class="pointer-events-none absolute inset-y-0 z-0 w-px bg-primary/25" style={`left: ${(todayIndex + 0.5) * dayWidthRem}rem`} data-roadmap-today-line></div>
								{/if}
								{#if bar.kind === 'span'}
									<button
										type="button"
										class={cn(buttonVariants({ variant: barVariant(row.task.status), size: 'sm' }), 'absolute top-1/2 z-[1] -mt-3.5 justify-start overflow-hidden active:not-aria-[haspopup]:translate-y-0', canUpdateTask(row.task) && 'cursor-grab active:cursor-grabbing', selected && 'ring-2 ring-ring', isTaskStatusCompleted(row.task.status) && 'line-through')}
										style={spanStyle(bar)}
										title={row.task.content}
										data-roadmap-bar={row.task.id}
										onclick={() => chooseTask(row.task)}
										use:taskRoadmapDrag={{
											enabled: canUpdateTask(row.task),
											dayWidth: dayWidthPixels,
											onPreview: (dayOffset) => previewMove(row.task, dayOffset),
											onCommit: (dayOffset) => void commitMove(row.task, dayOffset),
											onCancel: () => setMovedRange(row.task.id, undefined)
										}}
									>
										<span class="truncate">{row.task.content}</span>
									</button>
								{:else if bar.kind === 'before'}
									<button type="button" class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'absolute top-1/2 left-2 -mt-3.5')} onclick={() => scrollToDay(0)}>
										<ArrowLeftIcon />{row.task.content} · {shortDate(bar.dateISO)}
									</button>
								{:else if bar.kind === 'after'}
									<button type="button" class={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'absolute top-1/2 right-2 -mt-3.5')} onclick={() => scrollToDay(days.length)}>
										{row.task.content} · {shortDate(bar.dateISO)}<ArrowRightIcon />
									</button>
								{:else}
									<button type="button" class={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'absolute top-1/2 -mt-3.5 border-dashed border-border text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100')} style={`left: ${(Math.max(todayIndex, 0) + 1) * dayWidthRem}rem`} onclick={() => openTask(row.task)} data-roadmap-add-dates={row.task.id}>
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
