<script lang="ts">
	import './task-board-layout.css';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { MediaQuery } from 'svelte/reactivity';
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
	};

	type Props = {
		tasks: Task[];
		allTasks: Task[];
		serverChildProgress?: Record<string, TaskChildProgress>;
		boardText: BoardText;
		etcLabel: string;
		statusLabel: (status: string) => string;
		openTask: (task: Task) => void;
		createTask: (status?: string) => void;
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
		participantScope
	}: Props = $props();

	const columnClass = [
		'task-board-column group flex h-full min-h-0',
		'shrink-0 snap-start flex-col overflow-hidden rounded-lg border bg-muted/30',
		'max-sm:h-auto max-sm:overflow-visible max-sm:rounded-none max-sm:border-0 max-sm:bg-transparent'
	].join(' ');
	const boardScrollClass = [
		'h-[var(--task-board-height,32rem)] min-h-80 min-w-0',
		'overflow-x-auto overflow-y-hidden px-4 pb-2 scroll-px-4 md:px-8 md:scroll-px-8',
		'snap-x snap-mandatory',
		'max-sm:h-auto max-sm:min-h-0 max-sm:overflow-visible max-sm:snap-none max-sm:pb-0'
	].join(' ');
	const isCompact = new MediaQuery('(max-width: 639px)');
	const boardDrag = new TaskBoardDragController({
		isTaskPending,
		canUpdateTask: (task) => canUpdateTask(task),
		moveTask: (request) => moveTask(request)
	});

	let columns = $derived(buildTaskBoard(tasks, {
		weekStartISO,
		weekEndISO,
		weekPosition,
		hideEmptyRequestColumn: shouldHideEmptyRequestColumn(participantScope)
	}));
	let childProgressByParent = $derived(serverChildProgress ? new Map(Object.entries(serverChildProgress)) : buildTaskChildProgressByParent(allTasks));

	function addTaskLabel(status: string): string {
		return boardText.addTask.replace('{status}', statusLabel(status));
	}

	function isTaskPending(taskID: string): boolean {
		return pendingTaskIDs.includes(taskID);
	}

	function taskCountLabel(count: number): string {
		return `업무 ${count}개`;
	}

	function createTaskInColumn(status: string): void {
		if (!isTaskBoardStatus(status) || !canCreateTaskInColumn(status, participantScope)) return;
		createTask(status);
	}
</script>

<div class="sticky top-0 -mx-4 min-w-0 bg-background max-sm:static md:-mx-8">
	<div class={boardScrollClass} data-task-board-scroll use:taskBoardViewportHeight>
		<div class="flex h-full min-w-max gap-3 pr-4 max-sm:min-w-0 max-sm:flex-col max-sm:gap-3 max-sm:pr-0 md:pr-8">
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
					<header class="flex h-11 items-center justify-between gap-3 border-b bg-card px-3 max-sm:border-b-0 max-sm:bg-transparent max-sm:pl-1 max-sm:pr-0">
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
					</header>

					<div
						class={cn('min-h-0 flex-1 overflow-y-auto px-2.5 pb-3 pt-3 max-sm:overflow-visible max-sm:p-0', column.tasks.length === 0 && 'max-sm:hidden')}
						role="list"
						aria-label={statusLabel(column.status)}
						ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
						ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
					>
						<div class="space-y-2 max-sm:space-y-0 max-sm:overflow-hidden max-sm:rounded-lg max-sm:border max-sm:bg-card">
							{#each column.tasks as task (task.id)}
								<div role="listitem" class="max-sm:not-first:border-t">
									<TaskBoardCard
										{task}
										childProgress={childProgressByParent.get(task.id)}
										{memberEmail}
										isOverduePlan={isOverdueTaskPlan(task, weekStartISO)}
										{businessColor}
										{taskTypeColor}
										{childProgressLabel}
										{etcLabel}
										isCompact={isCompact.current}
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
									<span>업무 추가</span>
								</Button>
							</div>

						</div>
					</div>
				</section>
			{/each}
		</div>
	</div>
</div>
