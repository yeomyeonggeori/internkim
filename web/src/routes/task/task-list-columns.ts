import { renderComponent } from '$lib/components/ui/data-table';
import type { Column, ColumnDef } from '@tanstack/table-core';
import { compareOptionalDate } from './task-style';
import TaskListHeaderCell from './task-list-header-cell.svelte';
import DefinitionBadge from '$lib/components/definition-badge.svelte';
import TaskListBusinessCell from './task-list-business-cell.svelte';
import TaskListParticipantsCell from './task-list-participants-cell.svelte';
import TaskListSizeCell from './task-list-size-cell.svelte';
import TaskListStatusCell from './task-list-status-cell.svelte';
import TaskListTextCell from './task-list-text-cell.svelte';
import { taskDefinitionLabel } from './task-workspace-model';
import type { Task } from './task-types';
import { taskText } from './text';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

export type TaskListStatusOption = {
	value: string;
	label: string;
};

type TaskListColumnsInput = {
	text: TaskPageText;
	statusOptionsForTask: (task: Task) => TaskListStatusOption[];
	pendingStatusTaskID: string;
	statusLabel: (status: string) => string;
	updateTaskStatus: (task: Task, nextStatus: string) => Promise<void>;
	canUpdateTask: (task: Task) => boolean;
	memberEmail: (memberID: string) => string;
	businessColor: (business: string | null) => string;
	taskTypeColor: (type: string | null) => string;
};

export function createTaskListColumns(input: TaskListColumnsInput): ColumnDef<Task>[] {
	const {
		text,
		statusOptionsForTask,
		pendingStatusTaskID,
		statusLabel,
		updateTaskStatus,
		canUpdateTask,
		memberEmail,
		businessColor,
		taskTypeColor
	} = input;

	return [
		{
			accessorKey: 'business',
			header: (context) => renderHeader(text.table.business, context.column),
			cell: (info) => renderComponent(TaskListBusinessCell, {
				label: taskDefinitionLabel(info.row.original.business, text.report.etcLabel),
				color: businessColor(info.row.original.business)
			})
		},
		{
			accessorKey: 'type',
			header: (context) => renderHeader(text.table.type, context.column),
			cell: (info) => renderComponent(DefinitionBadge, {
				label: taskDefinitionLabel(info.row.original.type, text.report.etcLabel),
				color: taskTypeColor(info.row.original.type)
			})
		},
		{
			accessorKey: 'content',
			enableSorting: false,
			header: (context) => renderHeader(text.table.content, context.column),
			cell: (info) => renderComponent(TaskListTextCell, { value: info.row.original.content, isTruncated: true })
		},
		{
			id: 'participants',
			enableSorting: false,
			header: (context) => renderHeader(text.table.participants, context.column),
			cell: (info) => renderComponent(TaskListParticipantsCell, { task: info.row.original, memberEmail })
		},
		{
			accessorKey: 'size',
			header: (context) => renderHeader(text.table.size, context.column),
			cell: (info) => renderComponent(TaskListSizeCell, { task: info.row.original })
		},
		{
			accessorKey: 'status',
			header: (context) => renderHeader(text.table.status, context.column),
			cell: (info) => renderComponent(TaskListStatusCell, {
				task: info.row.original,
				statusOptionsForTask,
				pendingStatusTaskID,
				statusLabel,
				updateTaskStatus,
				canUpdateTask
			})
		},
		{
			accessorKey: 'startDate',
			header: (context) => renderHeader(text.table.startDate, context.column),
			cell: (info) => renderComponent(TaskListTextCell, { value: info.row.original.startDate || '-', isMuted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.startDate, right.original.startDate)
		},
		{
			accessorKey: 'endDate',
			header: (context) => renderHeader(text.table.endDate, context.column),
			cell: (info) => renderComponent(TaskListTextCell, { value: info.row.original.endDate || '-', isMuted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.endDate, right.original.endDate)
		}
	];
}

function renderHeader(label: string, column: Column<Task, unknown> | undefined) {
	return renderComponent(TaskListHeaderCell, { label, column });
}
