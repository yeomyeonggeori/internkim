import { renderComponent } from '$lib/components/ui/data-table';
import type { Column, ColumnDef } from '@tanstack/table-core';
import { compareOptionalDate } from './flow-style';
import FlowTaskListHeaderCell from './flow-task-list-header-cell.svelte';
import FlowTaskListBusinessCell from './flow-task-list-business-cell.svelte';
import FlowTaskListParticipantsCell from './flow-task-list-participants-cell.svelte';
import FlowTaskListSizeCell from './flow-task-list-size-cell.svelte';
import FlowTaskListStatusCell from './flow-task-list-status-cell.svelte';
import FlowTaskListTextCell from './flow-task-list-text-cell.svelte';
import { flowBusinessLabel } from './flow-task-workspace-model';
import type { FlowTask } from './flow-types';
import { flowText } from './text';

type FlowPageText = typeof flowText.ko;

export type FlowTaskListStatusOption = {
	value: string;
	label: string;
};

type FlowTaskListColumnsInput = {
	text: FlowPageText;
	statusOptions: FlowTaskListStatusOption[];
	pendingStatusTaskID: string;
	statusLabel: (status: string) => string;
	updateTaskStatus: (task: FlowTask, nextStatus: string) => Promise<void>;
	canUpdateTask: (task: FlowTask) => boolean;
	memberEmail: (memberID: string) => string;
	businessColor: (business: string) => string;
	taskTypeColor: (type: string) => string;
};

export function createFlowTaskListColumns(input: FlowTaskListColumnsInput): ColumnDef<FlowTask>[] {
	const {
		text,
		statusOptions,
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
			cell: (info) => renderComponent(FlowTaskListBusinessCell, {
				label: flowBusinessLabel(info.row.original.business, text.report.fallbackBusiness),
				color: businessColor(info.row.original.business)
			})
		},
		{
			accessorKey: 'type',
			header: (context) => renderHeader(text.table.type, context.column),
			cell: (info) => renderComponent(FlowTaskListBusinessCell, {
				label: info.row.original.type,
				color: taskTypeColor(info.row.original.type)
			})
		},
		{
			accessorKey: 'content',
			enableSorting: false,
			header: (context) => renderHeader(text.table.content, context.column),
			cell: (info) => renderComponent(FlowTaskListTextCell, { value: info.row.original.content, isTruncated: true })
		},
		{
			id: 'participants',
			enableSorting: false,
			header: (context) => renderHeader(text.table.participants, context.column),
			cell: (info) => renderComponent(FlowTaskListParticipantsCell, { task: info.row.original, memberEmail })
		},
		{
			accessorKey: 'size',
			header: (context) => renderHeader(text.table.size, context.column),
			cell: (info) => renderComponent(FlowTaskListSizeCell, { task: info.row.original })
		},
		{
			accessorKey: 'status',
			header: (context) => renderHeader(text.table.status, context.column),
			cell: (info) => renderComponent(FlowTaskListStatusCell, {
				task: info.row.original,
				statusOptions,
				pendingStatusTaskID,
				statusLabel,
				updateTaskStatus,
				canUpdateTask
			})
		},
		{
			accessorKey: 'startDate',
			header: (context) => renderHeader(text.table.startDate, context.column),
			cell: (info) => renderComponent(FlowTaskListTextCell, { value: info.row.original.startDate || '-', isMuted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.startDate, right.original.startDate)
		},
		{
			accessorKey: 'endDate',
			header: (context) => renderHeader(text.table.endDate, context.column),
			cell: (info) => renderComponent(FlowTaskListTextCell, { value: info.row.original.endDate || '-', isMuted: true }),
			sortingFn: (left, right) => compareOptionalDate(left.original.endDate, right.original.endDate)
		}
	];
}

function renderHeader(label: string, column: Column<FlowTask, unknown> | undefined) {
	return renderComponent(FlowTaskListHeaderCell, { label, column });
}
