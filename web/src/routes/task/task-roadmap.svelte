<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { onMount, untrack } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { taskBoardViewportHeight } from './task-board-viewport-height';
	import { hasRelatedTasks, relatedTaskIDs } from './task-roadmap-focus';
	import TaskRoadmapHeader from './task-roadmap-header.svelte';
	import { loadTaskRoadmapHolidays, noTaskRoadmapHolidays } from './task-roadmap-holidays';
	import { taskRoadmapRedDayBackground, type TaskRoadmapDaySpan } from './task-roadmap-layout';
	import {
		buildTaskRoadmapRows,
		clampTaskRoadmapMove,
		isTaskRoadmapWeekend,
		moveTaskDateRange,
		shiftTaskRoadmapDate,
		taskDateRange,
		taskRoadmapBar,
		taskRoadmapDays,
		taskRoadmapDaysBetween,
		type TaskDateRange,
		type TaskRoadmapBar
	} from './task-roadmap-model';
	import TaskRoadmapRow from './task-roadmap-row.svelte';
	import { slideTaskRoadmap } from './task-roadmap-slide';
	import { taskRoadmapSwipe, type TaskRoadmapSwipeDirection } from './task-roadmap-swipe-action';
	import {
		isTaskRoadmapStartAllowed,
		taskRoadmapNarrowDayCount,
		taskRoadmapWeekLength,
		taskRoadmapWeekStart,
		taskRoadmapWideDayCount,
		taskRoadmapWindowSpan
	} from './task-roadmap-window';
	import { buildTaskChildProgressByParent, type TaskChildProgress } from './task-relationships';
	import { taskDefinitionLabel } from './task-workspace-model';
	import type { Task } from './task-types';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskRoadmapText = PageText<typeof taskText>['task'];

	type Props = {
		tasks: Task[];
		allTasks: Task[];
		serverChildProgress?: Record<string, TaskChildProgress>;
		startISO: string;
		todayISO: string;
		text: TaskRoadmapText;
		statusLabel: (status: string) => string;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
		openTask: (task: Task) => void;
		canUpdateTask: (task: Task) => boolean;
		updateTaskDates: (task: Task, startDate: string, endDate: string) => Promise<boolean>;
		selectStart: (startISO: string) => void;
	};

	let { tasks, allTasks, serverChildProgress, startISO, todayISO, text, statusLabel, businessColor, taskTypeColor, openTask, canUpdateTask, updateTaskDates, selectStart }: Props = $props();

	const isNarrow = new IsMobile();
	const prefersReducedMotion = new MediaQuery('prefers-reduced-motion: reduce');
	let dayCount = $derived(isNarrow.current ? taskRoadmapNarrowDayCount : taskRoadmapWideDayCount);
	let days = $derived(taskRoadmapDays(startISO, dayCount));
	let todayIndex = $derived(days.indexOf(todayISO));
	let childProgressByParent = $derived(serverChildProgress ? new Map(Object.entries(serverChildProgress)) : buildTaskChildProgressByParent(allTasks));
	let rows = $derived(buildTaskRoadmapRows(tasks, days, childProgressByParent));
	let holidays = $state(noTaskRoadmapHolidays);
	let redDays = $derived(days.map((dayISO) => isTaskRoadmapWeekend(dayISO) || holidays.dates.has(dayISO)));
	let redDayBackground = $derived(taskRoadmapRedDayBackground(redDays));
	let selectedTaskID = $state('');
	let highlightedTaskIDs = $derived(selectedTaskID && hasRelatedTasks(selectedTaskID, allTasks) ? relatedTaskIDs(selectedTaskID, allTasks) : null);
	let movedRanges = $state(new Map<string, TaskDateRange>());
	let selectedRow = $derived(rows.find((row) => row.task.id === selectedTaskID));
	let selectedBar = $derived(selectedRow ? shownBar(selectedRow.task, selectedRow.bar) : undefined);
	let selectedSpan = $derived<TaskRoadmapDaySpan | undefined>(selectedBar?.kind === 'span' ? selectedBar : undefined);
	let frameElement = $state<HTMLElement | null>(null);
	let laneWidth = $state(0);
	let shownStartISO = '';

	onMount(() => {
		const span = taskRoadmapWindowSpan(todayISO);
		loadTaskRoadmapHolidays(span.firstDayISO, span.lastDayISO, text.roadmap.holidayLocale === 'en' ? 'en' : 'ko')
			.then((loaded) => { holidays = loaded; })
			.catch((error: unknown) => { console.warn('task roadmap holidays unavailable', { error }); });
	});

	$effect(() => {
		const nextStartISO = startISO;
		untrack(() => slideTo(nextStartISO));
	});

	function slideTo(nextStartISO: string): void {
		const movedDays = shownStartISO ? taskRoadmapDaysBetween(shownStartISO, nextStartISO) : 0;
		shownStartISO = nextStartISO;
		if (!frameElement || prefersReducedMotion.current) return;
		slideTaskRoadmap(frameElement, movedDays, dayCount, laneWidth);
	}

	function moveTo(nextStartISO: string): boolean {
		if (!isTaskRoadmapStartAllowed(nextStartISO, todayISO)) return false;
		selectStart(nextStartISO);
		return true;
	}

	function canMoveWeek(direction: TaskRoadmapSwipeDirection): boolean {
		return isTaskRoadmapStartAllowed(shiftTaskRoadmapDate(startISO, direction * taskRoadmapWeekLength), todayISO);
	}

	function moveWeek(direction: TaskRoadmapSwipeDirection): void {
		moveTo(shiftTaskRoadmapDate(startISO, direction * taskRoadmapWeekLength));
	}

	function jumpToTask(task: Task, dateISO: string): void {
		if (moveTo(taskRoadmapWeekStart(dateISO))) selectedTaskID = task.id;
	}

	function chooseBar(task: Task): void {
		if (selectedTaskID === task.id) {
			openTask(task);
			return;
		}
		selectedTaskID = task.id;
	}

	function clearSelectionOutsideTasks(event: MouseEvent): void {
		if (!selectedTaskID) return;
		const path = event.composedPath();
		if (path.some((target) => target instanceof Element && target.getAttribute('role') === 'dialog')) return;
		const insideFrame = frameElement !== null && path.includes(frameElement);
		if (insideFrame && path.some((target) => target instanceof HTMLButtonElement)) return;
		selectedTaskID = '';
	}

	function shownBar(task: Task, bar: TaskRoadmapBar): TaskRoadmapBar {
		const moved = movedRanges.get(task.id);
		return moved ? taskRoadmapBar({ ...task, ...moved }, days) : bar;
	}

	function setMovedRange(taskID: string, range: TaskDateRange | undefined): void {
		const next = new Map(movedRanges);
		if (range) next.set(taskID, range);
		else next.delete(taskID);
		movedRanges = next;
	}

	function previewMove(task: Task, dayOffset: number): void {
		const range = taskDateRange(task);
		if (range) setMovedRange(task.id, moveTaskDateRange(range, clampTaskRoadmapMove(range, days, dayOffset)));
	}

	async function commitMove(task: Task, dayOffset: number): Promise<void> {
		const range = taskDateRange(task);
		if (!range) return;
		const clampedOffset = clampTaskRoadmapMove(range, days, dayOffset);
		if (clampedOffset === 0) {
			setMovedRange(task.id, undefined);
			return;
		}
		const next = moveTaskDateRange(range, clampedOffset);
		setMovedRange(task.id, next);
		await updateTaskDates(task, next.startDate, next.endDate);
		setMovedRange(task.id, undefined);
	}
</script>

<svelte:window onclick={clearSelectionOutsideTasks} />

<div
	bind:this={frameElement}
	class="max-h-[var(--task-board-height,32rem)] touch-pan-y overflow-x-hidden overflow-y-auto overscroll-contain rounded-lg border"
	data-task-roadmap
	use:taskBoardViewportHeight
	use:taskRoadmapSwipe={{ onSwipe: moveWeek }}
>
	<TaskRoadmapHeader
		{days}
		{todayISO}
		{redDays}
		{holidays}
		{selectedSpan}
		selectedColor={selectedRow ? taskTypeColor(selectedRow.task.type) : ''}
		text={text.roadmap}
		canMoveBack={canMoveWeek(-1)}
		canMoveForward={canMoveWeek(1)}
		{moveWeek}
		chooseDay={moveTo}
		bind:laneWidth
	/>
	{#each rows as row (row.task.id)}
		<TaskRoadmapRow
			{row}
			bar={shownBar(row.task, row.bar)}
			{dayCount}
			{todayIndex}
			{redDayBackground}
			selected={selectedTaskID === row.task.id}
			dimmed={highlightedTaskIDs !== null && !highlightedTaskIDs.has(row.task.id)}
			typeLabel={taskDefinitionLabel(row.task.type, text.etcLabel)}
			typeColor={taskTypeColor(row.task.type)}
			businessColor={businessColor(row.task.business)}
			statusLabel={statusLabel(row.task.status)}
			{text}
			canUpdate={canUpdateTask(row.task)}
			chooseBar={() => chooseBar(row.task)}
			openTask={() => openTask(row.task)}
			jumpTo={(dateISO) => jumpToTask(row.task, dateISO)}
			previewMove={(dayOffset) => previewMove(row.task, dayOffset)}
			commitMove={(dayOffset) => void commitMove(row.task, dayOffset)}
			cancelMove={() => setMovedRange(row.task.id, undefined)}
		/>
	{/each}
</div>
