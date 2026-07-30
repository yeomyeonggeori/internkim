<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import FlowTaskBoardCard from './flow-task-board-card.svelte';
	import { FlowTaskBoardDragController } from './flow-task-board-drag-controller.svelte';
	import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
	import { flowTaskBoardViewportHeight } from './flow-task-board-viewport-height';
	import { buildFlowTaskBoard, isFlowTaskBoardStatus, isOverdueFlowPlan, type FlowTaskBoardWeekPosition } from './flow-task-board-model';
	import type { FlowTask } from './flow-types';

	type BoardText = {
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
		weekPosition?: FlowTaskBoardWeekPosition;
		memberEmail: (memberID: string) => string;
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
		weekEndISO = '',
		weekPosition = 'current',
		memberEmail
	}: Props = $props();

	const columnClass = [
		'flow-task-board-column group flex h-full min-h-0',
		'shrink-0 snap-start flex-col overflow-hidden rounded-lg border bg-muted/30'
	].join(' ');
	const boardScrollClass = [
		'h-[var(--flow-task-board-height,32rem)] min-h-80 min-w-0',
		'overflow-x-auto overflow-y-hidden px-4 pb-2 scroll-px-4 md:px-8 md:scroll-px-8',
		'snap-x snap-mandatory'
	].join(' ');
	const insertionLineWrapperClass = 'flex h-4 items-center px-1';
	const insertionLineClass = 'h-0.5 w-full rounded-full bg-primary shadow-sm ring-1 ring-primary/20';
	const boardDrag = new FlowTaskBoardDragController();

	let columns = $derived(buildFlowTaskBoard(tasks, { weekStartISO, weekEndISO, weekPosition }));

	$effect(() => {
		boardDrag.sync({ pendingTaskIDs, canUpdateTask, moveTask });
	});

	function addTaskLabel(status: string): string {
		return boardText.addTask.replace('{status}', statusLabel(status));
	}

	function isTaskPending(taskID: string): boolean {
		return boardDrag.isTaskPending(taskID);
	}

	function insertionIndicatorID(status: string, beforeTaskID: string): string {
		return `${status}:${beforeTaskID}`;
	}

	function taskCountLabel(count: number): string {
		return `업무 ${count}개`;
	}

	function createTaskInColumn(status: string): void {
		if (!isFlowTaskBoardStatus(status)) return;
		createTask(status);
	}
</script>

<div class="sticky top-0 -mx-4 min-w-0 bg-background md:-mx-8">
	<div class={boardScrollClass} data-flow-board-scroll use:flowTaskBoardViewportHeight>
		<div class="flex h-full min-w-max gap-3">
			{#each columns as column (column.status)}
				<section
					class={columnClass}
					role="group"
					aria-label={statusLabel(column.status)}
					data-flow-board-column={column.status}
					ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
					ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
				>
					<header class="flex h-11 items-center justify-between gap-3 border-b bg-card px-3">
						<div class="flex min-w-0 items-center gap-2">
							<span
								class="size-2.5 shrink-0 rounded-full border-2 bg-transparent"
								style:border-color={column.theme.accentColor}
							></span>
							<h3 class="truncate text-sm font-semibold text-foreground">{statusLabel(column.status)}</h3>
							<span
								class="inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-muted px-1.5 text-xs font-medium tabular-nums text-muted-foreground"
								aria-label={taskCountLabel(column.tasks.length)}
								data-flow-board-task-count
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
							onclick={() => createTaskInColumn(column.status)}
						>
							<PlusIcon class="size-3.5" />
						</Button>
					</header>

					<div
						class="min-h-0 flex-1 overflow-y-auto px-2.5 pb-3 pt-3"
						role="list"
						aria-label={statusLabel(column.status)}
						ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
						ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
					>
						<div class="space-y-2">
							{#each column.tasks as task (task.id)}
								{#if boardDrag.shouldShowCardInsertionLine(column.status, task.id)}
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
										{memberEmail}
										isOverduePlan={isOverdueFlowPlan(task, weekStartISO)}
										{businessFallback}
										{openTask}
										isPending={isTaskPending(task.id)}
										isReadOnly={!canUpdateTask(task)}
										onTaskDragStart={boardDrag.handleTaskDragStart}
										onTaskDragEnd={boardDrag.handleTaskDragEnd}
										onTaskDragOver={(event, value) => boardDrag.handleCardDragOver(event, column.status, column.tasks, value)}
										onTaskDrop={(event, value) => boardDrag.handleCardDrop(event, column.status, column.tasks, value)}
									/>
								</div>
							{/each}

							<div
								class="space-y-2"
								role="listitem"
								data-flow-board-drop-zone={column.status}
								ondragover={(event) => boardDrag.handleColumnDragOver(event, column.status, column.tasks)}
								ondrop={(event) => boardDrag.handleColumnDrop(event, column.status, column.tasks)}
							>
								{#if boardDrag.shouldShowAppendInsertionLine(column.status)}
									<div
										class={insertionLineWrapperClass}
										data-flow-board-drop-indicator={insertionIndicatorID(column.status, 'append')}
									>
										<div class={insertionLineClass}></div>
									</div>
								{/if}
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-8 w-full justify-center text-muted-foreground opacity-0 pointer-events-none transition-opacity hover:bg-muted hover:text-foreground focus-visible:opacity-100 focus-visible:pointer-events-auto group-hover:opacity-100 group-hover:pointer-events-auto group-focus-within:opacity-100 group-focus-within:pointer-events-auto"
									aria-label={addTaskLabel(column.status)}
									title={addTaskLabel(column.status)}
									data-flow-board-footer-add-task={column.status}
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

<style>
	[data-flow-board-scroll] {
		container-type: inline-size;
		scrollbar-width: none;
	}

	[data-flow-board-scroll]::-webkit-scrollbar {
		display: none;
	}

	.flow-task-board-column {
		width: max(13.5rem, calc((100cqw + 1.25rem) / 1.5));
	}

	@container (min-width: 560px) {
		.flow-task-board-column {
			width: max(13.5rem, calc((100cqw + 0.5rem) / 2.5));
		}
	}

	@container (min-width: 820px) {
		.flow-task-board-column {
			width: max(15rem, calc((100cqw - 0.25rem) / 3.5));
		}
	}

	@container (min-width: 1180px) {
		.flow-task-board-column {
			width: max(15.5rem, calc((100cqw - 1rem) / 4.5));
		}
	}
</style>
