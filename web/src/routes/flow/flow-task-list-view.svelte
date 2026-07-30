<script lang="ts">
	import { createSvelteTable } from '$lib/components/ui/data-table';
	import {
		type ColumnDef,
		type PaginationState,
		type Row,
		type SortingState,
		getCoreRowModel,
		getPaginationRowModel,
		getSortedRowModel
	} from '@tanstack/table-core';
	import { createFlowTaskListColumns, type FlowTaskListStatusOption } from './flow-task-list-columns';
	import FlowTaskTable from './flow-task-table.svelte';
	import type { FlowTask } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Props = {
		tasks: FlowTask[];
		text: FlowPageText;
		statusOptions: FlowTaskListStatusOption[];
		pendingStatusTaskID: string;
		statusLabel: (status: string) => string;
		updateTaskStatus: (task: FlowTask, nextStatus: string) => Promise<void>;
		openTask: (task: FlowTask) => void;
		canUpdateTask: (task: FlowTask) => boolean;
		focusedTaskID: string;
		memberEmail: (memberID: string) => string;
		businessColor: (business: string) => string;
		taskTypeColor: (type: string) => string;
		sizeColor: (size: string) => string;
	};

	let {
		tasks,
		text,
		statusOptions,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		openTask,
		canUpdateTask,
		focusedTaskID,
		memberEmail,
		businessColor,
		taskTypeColor,
		sizeColor
	}: Props = $props();

	let taskSorting = $state<SortingState>([]);
	let taskPagination = $state<PaginationState>({ pageIndex: 0, pageSize: 20 });

	let taskColumns: ColumnDef<FlowTask>[] = $derived(createFlowTaskListColumns({
		text,
		statusOptions,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		canUpdateTask,
		memberEmail,
		businessColor,
		taskTypeColor,
		sizeColor
	}));

	const taskTable = createSvelteTable<FlowTask>({
		get data() {
			return tasks;
		},
		get columns() {
			return taskColumns;
		},
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
		state: {
			get sorting() {
				return taskSorting;
			},
			get pagination() {
				return taskPagination;
			}
		},
		onSortingChange: (updater) => {
			taskSorting = typeof updater === 'function' ? updater(taskSorting) : updater;
		},
		onPaginationChange: (updater) => {
			taskPagination = typeof updater === 'function' ? updater(taskPagination) : updater;
		}
	});

	$effect(() => {
		showFocusedTaskPage(focusedTaskID);
	});

	function showFocusedTaskPage(taskID: string) {
		if (!taskID) return;
		const rowIndex = taskTable.getSortedRowModel().rows.findIndex((row: Row<FlowTask>) => row.original.id === taskID);
		if (rowIndex < 0) return;
		const pageIndex = Math.floor(rowIndex / taskPagination.pageSize);
		if (pageIndex === taskPagination.pageIndex) return;
		taskPagination = { ...taskPagination, pageIndex };
	}
</script>

<FlowTaskTable
	{taskTable}
	columnCount={taskColumns.length}
	pageSize={taskPagination.pageSize}
	emptyLabel={text.task.empty}
	pagination={text.table.pagination}
	{openTask}
	{focusedTaskID}
/>
