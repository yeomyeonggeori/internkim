<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { createSvelteTable, renderSnippet } from '$lib/components/ui/data-table';
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import {
		type ColumnDef,
		type PaginationState,
		type SortingState,
		getCoreRowModel,
		getPaginationRowModel,
		getSortedRowModel
	} from '@tanstack/table-core';
	import { compareOptionalDate, sizeBadgeClass, statusBadgeClass } from './flow-style';
	import FlowTaskTable from './flow-task-table.svelte';
	import { flowBusinessLabel } from './flow-task-workspace-model';
	import type { FlowTask } from './flow-types';
	import { flowText } from './text';

	type FlowPageText = typeof flowText.ko;

	type Option = {
		value: string;
		label: string;
	};

	type Props = {
		tasks: FlowTask[];
		text: FlowPageText;
		statusOptions: Option[];
		pendingStatusTaskID: string;
		statusLabel: (status: string) => string;
		updateTaskStatus: (task: FlowTask, nextStatus: string) => Promise<void>;
		openTask: (task: FlowTask) => void;
		canUpdateTask: (task: FlowTask) => boolean;
		focusedTaskID: string;
	};

	let { tasks, text, statusOptions, pendingStatusTaskID, statusLabel, updateTaskStatus, openTask, canUpdateTask, focusedTaskID }: Props = $props();

	let taskSorting = $state<SortingState>([]);
	let taskPagination = $state<PaginationState>({ pageIndex: 0, pageSize: 20 });

	const taskColumns: ColumnDef<FlowTask>[] = [
		{
			accessorKey: 'ownerName',
			header: () => renderSnippet(taskHeader, { label: text.table.owner, id: 'ownerName' }),
			cell: (info) => renderSnippet(taskOwnerCell, { task: info.row.original })
		},
		{
			accessorKey: 'business',
			header: () => renderSnippet(taskHeader, { label: text.table.business, id: 'business' }),
			cell: (info) => renderSnippet(taskTextCell, { value: flowBusinessLabel(info.row.original.business, text.report.fallbackBusiness), muted: true })
		},
		{
			accessorKey: 'type',
			header: () => renderSnippet(taskHeader, { label: text.table.type, id: 'type' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.type, muted: false })
		},
		{
			accessorKey: 'content',
			enableSorting: false,
			header: () => renderSnippet(taskHeader, { label: text.table.content, id: 'content' }),
			cell: (info) => renderSnippet(taskContentCell, { value: info.row.original.content })
		},
		{
			id: 'participants',
			enableSorting: false,
			header: () => renderSnippet(taskHeader, { label: text.table.participants, id: 'participants' }),
			cell: (info) => renderSnippet(taskParticipantsCell, { task: info.row.original })
		},
		{
			accessorKey: 'size',
			header: () => renderSnippet(taskHeader, { label: text.table.size, id: 'size' }),
			cell: (info) => renderSnippet(taskSizeCell, { task: info.row.original })
		},
		{
			accessorKey: 'status',
			header: () => renderSnippet(taskHeader, { label: text.table.status, id: 'status' }),
			cell: (info) => renderSnippet(taskStatusCell, { task: info.row.original })
		},
		{
			accessorKey: 'startDate',
			header: () => renderSnippet(taskHeader, { label: text.table.startDate, id: 'startDate' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.startDate || '-', muted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.startDate, right.original.startDate)
		},
		{
			accessorKey: 'endDate',
			header: () => renderSnippet(taskHeader, { label: text.table.endDate, id: 'endDate' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.endDate || '-', muted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.endDate, right.original.endDate)
		},
		{
			accessorKey: 'flag',
			header: () => renderSnippet(taskHeader, { label: text.table.flag, id: 'flag', align: 'right' }),
			cell: (info) => renderSnippet(taskNumberCell, { value: info.row.original.flag })
		}
	];

	const taskTable = createSvelteTable<FlowTask>({
		get data() {
			return tasks;
		},
		columns: taskColumns,
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
		const rowIndex = taskTable.getSortedRowModel().rows.findIndex((row) => row.original.id === taskID);
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

{#snippet taskHeader({ label, id, align }: { label: string; id: string; align?: 'left' | 'right' })}
	{@const column = taskTable.getColumn(id)}
	{@const sortDirection = column?.getIsSorted()}
	{@const canSort = column?.getCanSort()}
	<div class={cn('flex items-center gap-1', align === 'right' && 'justify-end')}>
		{#if canSort}
			<button
				type="button"
				class="-mx-1 inline-flex items-center gap-1 rounded px-1 py-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground hover:bg-muted hover:text-foreground"
				onclick={() => column?.toggleSorting(sortDirection === 'asc')}
			>
				{label}
				{#if sortDirection === 'asc'}
					<ArrowUpIcon class="size-3" />
				{:else if sortDirection === 'desc'}
					<ArrowDownIcon class="size-3" />
				{:else}
					<ArrowUpDownIcon class="size-3 opacity-40" />
				{/if}
			</button>
		{:else}
			<span class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
		{/if}
	</div>
{/snippet}

{#snippet taskOwnerCell({ task }: { task: FlowTask })}
	<div class="flex items-center gap-2 font-medium">
		<PersonAvatar name={task.ownerName} seed={task.ownerID || task.ownerName} class="size-6" />
		{task.ownerName}
	</div>
{/snippet}

{#snippet taskTextCell({ value, muted }: { value: string; muted: boolean })}
	<span class={muted ? 'text-muted-foreground' : ''}>{value}</span>
{/snippet}

{#snippet taskContentCell({ value }: { value: string })}
	<span class="block max-w-[26rem] truncate">{value}</span>
{/snippet}

{#snippet taskParticipantsCell({ task }: { task: FlowTask })}
	<div class="flex max-w-56 flex-wrap gap-1">
		{#each task.participantNames as name, index}
			<Badge variant="outline" class="gap-1.5 pl-1">
				<PersonAvatar name={name} seed={task.participantIDs[index] ?? name} class="size-4" />
				{name}
			</Badge>
		{/each}
	</div>
{/snippet}

{#snippet taskSizeCell({ task }: { task: FlowTask })}
	<Badge class={sizeBadgeClass(task.size)}>{task.size}</Badge>
{/snippet}

{#snippet taskStatusCell({ task }: { task: FlowTask })}
	<div onclick={(event) => event.stopPropagation()} role="presentation">
		<Select.Root
			type="single"
			value={task.status}
			disabled={pendingStatusTaskID === task.id || !canUpdateTask(task)}
			onValueChange={(next) => updateTaskStatus(task, next)}
		>
			<Select.Trigger
				size="sm"
				class={cn('w-28 justify-between border-transparent font-medium', statusBadgeClass(task.status))}
			>
				{statusLabel(task.status)}
			</Select.Trigger>
			<Select.Content>
				{#each statusOptions as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
{/snippet}

{#snippet taskNumberCell({ value }: { value: number })}
	<span class="block text-right tabular-nums">{value}</span>
{/snippet}
