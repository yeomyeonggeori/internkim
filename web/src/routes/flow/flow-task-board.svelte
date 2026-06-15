<!-- Flow 업무를 상태 컬럼 보드로 표시합니다. -->
<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import FlowTaskBoardCard from './flow-task-board-card.svelte';
	import { buildFlowTaskBoard, isFlowTaskBoardStatus } from './flow-task-board-model';
	import type { FlowTask } from './flow-types';

	type BoardText = {
		emptyColumn: string;
		addTask: string;
	};

	type Props = {
		tasks: FlowTask[];
		boardText: BoardText;
		statusLabel: (status: string) => string;
		openTask: (task: FlowTask) => void;
		createTask: (status?: string) => void;
	};

	let {
		tasks,
		boardText,
		statusLabel,
		openTask,
		createTask
		}: Props = $props();

	const columnClass = [
		'group flex h-[min(48rem,calc(100vh-6rem))] min-h-[32rem]',
		'w-80 shrink-0 flex-col rounded-lg border bg-muted/30'
	].join(' ');
	const addTaskButtonClass = [
		'h-8 w-full justify-center border border-dashed border-muted-foreground/30',
		'text-muted-foreground opacity-0 transition-opacity',
		'hover:border-primary/40 hover:text-primary focus-visible:opacity-100',
		'group-focus-within:opacity-100 group-hover:opacity-100'
	].join(' ');

	let columns = $derived(buildFlowTaskBoard(tasks));

	function addTaskLabel(status: string): string {
		return boardText.addTask.replace('{status}', statusLabel(status));
	}
</script>

<div class="overflow-x-auto pb-2">
	<div class="flex min-w-max gap-3">
			{#each columns as column (column.status)}
				<section
					class={columnClass}
					role="group"
					aria-label={statusLabel(column.status)}
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

				<div class="min-h-0 flex-1 overflow-y-auto p-2">
					<div class="space-y-2">
						{#each column.tasks as task (task.id)}
							<FlowTaskBoardCard
								{task}
								{openTask}
							/>
						{/each}

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
