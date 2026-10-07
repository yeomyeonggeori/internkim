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
	import { createTaskListColumns, type TaskListStatusOption } from './task-list-columns';
	import TaskTable from './task-table.svelte';
	import type { Task } from './task-types';
	import { taskText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type TaskPageText = PageText<typeof taskText>;

	type Props = {
		tasks: Task[];
		emptyLabel: string;
		showEmpty: boolean;
		text: TaskPageText;
		statusOptionsForTask: (task: Task) => TaskListStatusOption[];
		pendingStatusTaskID: string;
		statusLabel: (status: string) => string;
		updateTaskStatus: (task: Task, nextStatus: string) => Promise<void>;
		openTask: (task: Task) => void;
		canUpdateTask: (task: Task) => boolean;
		focusedTaskID: string;
		memberEmail: (memberID: string) => string;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
	};

	let {
		tasks,
		emptyLabel,
		showEmpty,
		text,
		statusOptionsForTask,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		openTask,
		canUpdateTask,
		focusedTaskID,
		memberEmail,
		businessColor,
		taskTypeColor
	}: Props = $props();

	let taskSorting = $state<SortingState>([]);
	let taskPagination = $state<PaginationState>({ pageIndex: 0, pageSize: 20 });

	let taskColumns: ColumnDef<Task>[] = $derived(createTaskListColumns({
		text,
		statusOptionsForTask,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		canUpdateTask,
		memberEmail,
		businessColor,
		taskTypeColor
	}));

	const taskTable = createSvelteTable<Task>({
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
		const rowIndex = taskTable.getSortedRowModel().rows.findIndex((row: Row<Task>) => row.original.id === taskID);
		if (rowIndex < 0) return;
		const pageIndex = Math.floor(rowIndex / taskPagination.pageSize);
		if (pageIndex === taskPagination.pageIndex) return;
		taskPagination = { ...taskPagination, pageIndex };
	}
</script>

<TaskTable
	mobileLabels={text.table}
	{businessColor}
	{taskTable}
	columnCount={taskColumns.length}
	pageSize={taskPagination.pageSize}
	{emptyLabel}
	{showEmpty}
	pagination={text.table.pagination}
	{openTask}
	{focusedTaskID}
/>
