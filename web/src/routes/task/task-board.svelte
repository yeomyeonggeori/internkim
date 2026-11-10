<script lang="ts">
	import './task-board-layout.css';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TaskBoardCard from './task-board-card.svelte';
	import { TaskBoardDragController } from './task-board-drag-controller.svelte';
	import type { TaskBoardMoveRequest } from './task-board-drag';
	import { taskBoardViewportHeight } from './task-board-viewport-height';
	import { buildTaskBoard, isTaskBoardStatus, isOverdueTaskPlan, type TaskBoardWeekPosition } from './task-board-model';
	import { taskStatusIcon } from './task-status-style';
	import { statusIconClass } from './task-style';
	import { buildTaskChildProgressByParent, type TaskChildProgress } from './task-relationships';
	import {
		canCreateTaskInColumn,
		shouldHideEmptyRequestColumn,
		type TaskBoardParticipantScope
	} from './task-board-participant-scope';
	import type { Task } from './task-types';

	type BoardText = {
		addTask: string;
		addTaskShort: string;
		taskCount: string;
	};

	type Props = {
		tasks: Task[];
		allTasks: Task[];
		serverChildProgress?: Record<string, TaskChildProgress>;
		boardText: BoardText;
		etcLabel: string;
		statusLabel: (status: string) => string;
		openTask: (task: Task) => void;
		createTask?: (status?: string) => void;
		moveTask: (request: TaskBoardMoveRequest) => void | Promise<void>;
		pendingTaskIDs: string[];
		canUpdateTask: (task: Task) => boolean;
		weekStartISO?: string;
		weekEndISO?: string;
		weekPosition?: TaskBoardWeekPosition;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
		childProgressLabel: string;
		memberEmail: (memberID: string) => string;
		participantScope: TaskBoardParticipantScope;
		fitsViewport?: boolean;
	};

	let {
		tasks,
		allTasks,
		serverChildProgress,
		boardText,
		etcLabel,
		statusLabel,
		openTask,
		createTask,
		moveTask,
		pendingTaskIDs,
		canUpdateTask,
		weekStartISO = '',
		weekEndISO = '',
		weekPosition = 'current',
		businessColor,
		taskTypeColor,
		childProgressLabel,
		memberEmail,
		participantScope,
		fitsViewport = true
	}: Props = $props();

	const columnClass = [
		'task-board-column group flex h-full min-h-0',
		'shrink-0 snap-start flex-col overflow-hidden rounded-lg border bg-muted/30',
		'max-sm:h-auto'
	].join(' ');
	const boardScrollClass = [
		'h-[var(--task-board-height,32rem)] min-h-80 min-w-0',
		'overflow-x-auto overflow-y-hidden px-[var(--task-board-gutter,1rem)] pb-2 scroll-px-[var(--task-board-gutter,1rem)] md:px-[var(--task-board-gutter,2rem)] md:scroll-px-[var(--task-board-gutter,2rem)]',
		'snap-x snap-mandatory',
		'max-sm:h-auto max-sm:min-h-0 max-sm:overflow-visible max-sm:snap-none max-sm:pb-0'
	].join(' ');
	const boardDrag = new TaskBoardDragController({
		isTaskPending,
		canUpdateTask: (task) => canUpdateTask(task),
		moveTask: (request) => moveTask(request)
	});

	let childProgressByParent = $derived(serverChildProgress ? new Map(Object.entries(serverChildProgress)) : buildTaskChildProgressByParent(allTasks));
	let columns = $derived(buildTaskBoard(tasks, {
		weekStartISO,
		weekEndISO,
		weekPosition,
		hideEmptyRequestColumn: shouldHideEmptyRequestColumn(participantScope),
		parentTaskIDs: new Set(childProgressByParent.keys())
	}));

	function addTaskLabel(status: string): string {
		return boardText.addTask.replace('{status}', statusLabel(status));
	}

	function isTaskPending(taskID: string): boolean {
		return pendingTaskIDs.includes(taskID);
	}

	function taskCountLabel(count: number): string {
		return boardText.taskCount.replace('{count}', String(count));
	}

	function createTaskInColumn(status: string): void {
		if (!createTask || !isTaskBoardStatus(status) || !canCreateTaskInColumn(status, participantScope)) return;
		createTask(status);
	}
</script>

<div class="sticky top-0 -mx-[var(--task-board-gutter,1rem)] min-w-0 bg-background max-sm:static md:-mx-[var(--task-board-gutter,2rem)]">
	<div class={boardScrollClass} data-task-board-scroll use:taskBoardViewportHeight={fitsViewport}>
		<div class="flex h-full min-w-max gap-3 pr-[var(--task-board-gutter,1rem)] max-sm:min-w-0 max-sm:flex-col max-sm:gap-3 max-sm:pr-0 md:pr-[var(--task-board-gutter,2rem)]">
			{#each columns as column (column.status)}
				{@const StatusIcon = taskStatusIcon(column.status)}
				<section
					class={columnClass}
					role="group"
					aria-label={statusLabel(column.status)}
					data-task-board-column={column.status}
					ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
					ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
				>
					<header class="flex h-11 items-center justify-between gap-3 border-b bg-card px-3">
						<div class="flex min-w-0 items-center gap-2">
							<StatusIcon class={cn('size-4 shrink-0', statusIconClass(column.status))} aria-hidden="true" />
							<h3 class="truncate text-sm font-semibold text-foreground">{statusLabel(column.status)}</h3>
							<span
								class={cn(
									'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full px-1.5 text-xs font-medium tabular-nums',
									column.status === 'requested' && column.tasks.length > 0
										? 'bg-destructive text-background'
										: 'bg-muted text-muted-foreground'
								)}
								aria-label={taskCountLabel(column.tasks.length)}
								data-task-board-task-count
							>
								{column.tasks.length}
							</span>
						</div>
						{#if createTask}
							<Button
								type="button"
								variant="ghost"
								size="icon-xs"
								class="shrink-0"
								aria-label={addTaskLabel(column.status)}
								title={addTaskLabel(column.status)}
								disabled={!canCreateTaskInColumn(column.status, participantScope)}
								onclick={() => createTaskInColumn(column.status)}
							>
								<PlusIcon class="size-3.5" />
							</Button>
						{/if}
					</header>

					<div
						class={cn('min-h-0 flex-1 overflow-y-auto px-2.5 pb-3 pt-3 max-sm:overflow-visible', column.tasks.length === 0 && 'max-sm:hidden')}
						role="list"
						aria-label={statusLabel(column.status)}
						ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
						ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
					>
						<div class="space-y-2">
							{#each column.tasks as task (task.id)}
								<div role="listitem">
									<TaskBoardCard
										{task}
										childProgress={childProgressByParent.get(task.id)}
										{memberEmail}
										isOverduePlan={isOverdueTaskPlan(task, weekStartISO)}
										{businessColor}
										{taskTypeColor}
										{childProgressLabel}
										{etcLabel}
										{openTask}
										isPending={isTaskPending(task.id)}
										isReadOnly={!canUpdateTask(task)}
										onTaskDragStart={boardDrag.handleTaskDragStart}
										onTaskDragEnd={boardDrag.handleTaskDragEnd}
									/>
								</div>
							{/each}

							<div
								class="space-y-2 max-sm:hidden"
								role="listitem"
								data-task-board-drop-zone={column.status}
								ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
								ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
							>
								{#if createTask}
									<Button
										type="button"
										variant="ghost"
										size="sm"
										class="h-8 w-full justify-center text-muted-foreground transition-opacity hover:bg-muted hover:text-foreground focus-visible:opacity-100 focus-visible:pointer-events-auto sm:opacity-0 sm:pointer-events-none group-hover:opacity-100 group-hover:pointer-events-auto group-focus-within:opacity-100 group-focus-within:pointer-events-auto"
										aria-label={addTaskLabel(column.status)}
										title={addTaskLabel(column.status)}
										data-task-board-footer-add-task={column.status}
										disabled={!canCreateTaskInColumn(column.status, participantScope)}
										onclick={() => createTaskInColumn(column.status)}
									>
										<PlusIcon class="size-4" />
										<span>{boardText.addTaskShort}</span>
									</Button>
								{/if}
							</div>

						</div>
					</div>
				</section>
			{/each}
		</div>
	</div>
</div>
