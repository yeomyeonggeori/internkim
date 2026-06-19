<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import FlowTaskBoardCard from './flow-task-board-card.svelte';
	import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
	import { flowTaskBoardViewportHeight } from './flow-task-board-viewport-height';
	import { buildFlowTaskBoard, isFlowTaskBoardStatus } from './flow-task-board-model';
	import type { FlowTask } from './flow-types';

	type BoardText = {
		emptyColumn: string;
		addTask: string;
	};

	type Props = {
		tasks: FlowTask[];
		boardText: BoardText;
		businessFallback: string;
		statusLabel: (status: string) => string;
		openTask: (task: FlowTask) => void;
		createTask: (status?: string) => void;
		moveTask: (request: FlowTaskBoardMoveRequest) => void | Promise<void>;
		pendingTaskIDs: string[];
		canUpdateTask: (task: FlowTask) => boolean;
		weekStartISO?: string;
		weekEndISO?: string;
	};

	let {
		tasks,
		boardText,
		businessFallback,
		statusLabel,
		openTask,
		createTask,
		moveTask,
		pendingTaskIDs,
		canUpdateTask,
		weekStartISO = '',
		weekEndISO = ''
	}: Props = $props();

	const columnClass = [
		'group flex h-[var(--flow-task-board-height,32rem)] max-h-[var(--flow-task-board-height,32rem)] min-h-80',
		'w-80 shrink-0 flex-col rounded-lg border bg-muted/30'
	].join(' ');
	const addTaskButtonClass = [
		'h-8 w-full justify-center border border-dashed border-muted-foreground/30',
		'text-muted-foreground opacity-0 transition-opacity',
		'hover:border-primary/40 hover:text-primary focus-visible:opacity-100',
		'group-hover:opacity-100'
	].join(' ');
	const insertionLineWrapperClass = 'flex h-4 items-center px-1';
	const insertionLineClass = 'h-0.5 w-full rounded-full bg-primary shadow-sm ring-1 ring-primary/20';
	const boardDragDataType = 'application/x-internkim-flow-task-id';

	let columns = $derived(buildFlowTaskBoard(tasks, { weekStartISO, weekEndISO }));
	let draggedTaskID = $state('');
	let dropTarget = $state<FlowTaskBoardMoveRequest | null>(null);

	function addTaskLabel(status: string): string {
		return boardText.addTask.replace('{status}', statusLabel(status));
	}

	function isTaskPending(taskID: string): boolean {
		return pendingTaskIDs.includes(taskID);
	}

	function handleTaskDragStart(event: DragEvent, task: FlowTask): void {
		if (isTaskPending(task.id) || !canUpdateTask(task)) {
			event.preventDefault();
			return;
		}
		draggedTaskID = task.id;
		event.dataTransfer?.setData(boardDragDataType, task.id);
		event.dataTransfer?.setData('text/plain', task.id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
	}

	function handleTaskDragEnd(): void {
		draggedTaskID = '';
		dropTarget = null;
	}

	function handleColumnDragOver(event: DragEvent, status: string, columnTasks: FlowTask[]): void {
		const taskID = currentDragTaskID(event);
		if (!taskID || isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return;
		if (isSameColumnDropNoOp(taskID, null, columnTasks)) {
			dropTarget = null;
			return;
		}
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		dropTarget = { taskID, targetStatus: status, beforeTaskID: null };
	}

	function handleColumnDrop(event: DragEvent, status: string, columnTasks: FlowTask[]): void {
		event.preventDefault();
		const taskID = currentDragTaskID(event);
		if (!taskID || isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return;
		if (isSameColumnDropNoOp(taskID, null, columnTasks)) {
			handleTaskDragEnd();
			return;
		}
		void moveTask({ taskID, targetStatus: status, beforeTaskID: null });
		handleTaskDragEnd();
	}

	function handleCardDragOver(event: DragEvent, status: string, columnTasks: FlowTask[], task: FlowTask): void {
		const nextDropTarget = cardDropTarget(event, status, columnTasks, task);
		if (!nextDropTarget) {
			if (currentDragTaskID(event)) {
				event.stopPropagation();
				dropTarget = null;
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		dropTarget = nextDropTarget;
	}

	function handleCardDrop(event: DragEvent, status: string, columnTasks: FlowTask[], task: FlowTask): void {
		const nextDropTarget = cardDropTarget(event, status, columnTasks, task);
		if (!nextDropTarget) {
			if (currentDragTaskID(event)) {
				event.preventDefault();
				event.stopPropagation();
				handleTaskDragEnd();
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		void moveTask(nextDropTarget);
		handleTaskDragEnd();
	}

	function cardDropTarget(
		event: DragEvent,
		status: string,
		columnTasks: FlowTask[],
		task: FlowTask
	): FlowTaskBoardMoveRequest | null {
		const taskID = currentDragTaskID(event);
		if (!taskID || taskID === task.id || isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return null;
		const currentTarget = event.currentTarget;
		if (!(currentTarget instanceof HTMLElement)) return null;
		const bounds = currentTarget.getBoundingClientRect();
		const isAfterTask = event.clientY > bounds.top + bounds.height / 2;
		const taskIndex = columnTasks.findIndex((candidate) => candidate.id === task.id);
		if (taskIndex < 0) return null;
		const nextTask = isAfterTask ? columnTasks[taskIndex + 1] : task;
		const beforeTaskID = nextTask?.id ?? null;
		if (isSameColumnDropNoOp(taskID, beforeTaskID, columnTasks)) return null;
		return { taskID, targetStatus: status, beforeTaskID };
	}

	function currentDragTaskID(event: DragEvent): string {
		return draggedTaskID || event.dataTransfer?.getData(boardDragDataType) || '';
	}

	function isSameColumnDropNoOp(taskID: string, beforeTaskID: string | null, columnTasks: FlowTask[]): boolean {
		const draggedTaskIndex = columnTasks.findIndex((task) => task.id === taskID);
		if (draggedTaskIndex < 0) return false;
		if (beforeTaskID === null) return draggedTaskIndex === columnTasks.length - 1;
		const beforeTaskIndex = columnTasks.findIndex((task) => task.id === beforeTaskID);
		return beforeTaskIndex === draggedTaskIndex || beforeTaskIndex === draggedTaskIndex + 1;
	}

	function shouldShowCardInsertionLine(status: string, taskID: string): boolean {
		return dropTarget?.targetStatus === status && dropTarget.beforeTaskID === taskID;
	}

	function shouldShowAppendInsertionLine(status: string): boolean {
		return dropTarget?.targetStatus === status && dropTarget.beforeTaskID === null;
	}

	function insertionIndicatorID(status: string, beforeTaskID: string): string {
		return `${status}:${beforeTaskID}`;
	}
</script>

<div class="overflow-x-auto pb-2" data-flow-board-scroll use:flowTaskBoardViewportHeight>
	<div class="flex min-w-max gap-3">
		{#each columns as column (column.status)}
			<section
				class={columnClass}
				role="group"
				aria-label={statusLabel(column.status)}
				data-flow-board-column={column.status}
				ondragover={(event) => handleColumnDragOver(event, column.status, column.tasks)}
				ondrop={(event) => handleColumnDrop(event, column.status, column.tasks)}
			>
				<header class={`flex items-center justify-between gap-2 border-b px-3 py-2 ${column.theme.headerClass}`}>
					<div class="flex min-w-0 items-center gap-2">
						<span class={`size-2.5 shrink-0 rounded-full ${column.theme.dotClass}`}></span>
						<h3 class={`truncate text-sm font-semibold ${column.theme.titleClass}`}>{statusLabel(column.status)}</h3>
					</div>
					<span class="rounded-full bg-background px-2 py-0.5 text-xs tabular-nums text-muted-foreground">
						{column.tasks.length}
					</span>
				</header>

				<div
					class="min-h-0 flex-1 overflow-y-auto p-2"
					role="list"
					aria-label={statusLabel(column.status)}
					ondragover={(event) => handleColumnDragOver(event, column.status, column.tasks)}
					ondrop={(event) => handleColumnDrop(event, column.status, column.tasks)}
				>
					<div class="space-y-2">
						{#each column.tasks as task (task.id)}
							{#if shouldShowCardInsertionLine(column.status, task.id)}
								<div
									class={insertionLineWrapperClass}
									data-flow-board-drop-indicator={insertionIndicatorID(column.status, task.id)}
								>
									<div class={insertionLineClass}></div>
								</div>
							{/if}

							<div role="listitem">
									<FlowTaskBoardCard
										{task}
										{businessFallback}
										{openTask}
									isPending={isTaskPending(task.id)}
									isReadOnly={!canUpdateTask(task)}
									onTaskDragStart={handleTaskDragStart}
									onTaskDragEnd={handleTaskDragEnd}
									onTaskDragOver={(event, value) => handleCardDragOver(event, column.status, column.tasks, value)}
									onTaskDrop={(event, value) => handleCardDrop(event, column.status, column.tasks, value)}
								/>
							</div>
						{/each}

						<div
							class="min-h-4"
							role="presentation"
							data-flow-board-drop-zone={column.status}
							ondragover={(event) => handleColumnDragOver(event, column.status, column.tasks)}
							ondrop={(event) => handleColumnDrop(event, column.status, column.tasks)}
						>
							{#if shouldShowAppendInsertionLine(column.status)}
								<div
									class={insertionLineWrapperClass}
									data-flow-board-drop-indicator={insertionIndicatorID(column.status, 'append')}
								>
									<div class={insertionLineClass}></div>
								</div>
							{/if}
						</div>

						{#if column.tasks.length === 0}
							<div class="rounded-md border border-dashed px-3 py-8 text-center text-sm text-muted-foreground">
								{boardText.emptyColumn}
							</div>
						{/if}

						<Button
							type="button"
							variant="ghost"
							class={addTaskButtonClass}
							aria-label={addTaskLabel(column.status)}
							title={addTaskLabel(column.status)}
							onclick={() => {
								if (isFlowTaskBoardStatus(column.status)) createTask(column.status);
							}}
						>
							<PlusIcon class="size-4" />
						</Button>
					</div>
				</div>
			</section>
		{/each}
	</div>
</div>
