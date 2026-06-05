<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { FlexRender } from '$lib/components/ui/data-table';
	import * as Table from '$lib/components/ui/table';
	import type { Table as TableInstance } from '@tanstack/table-core';
	import type { FlowTask } from './flow-types';

	type PaginationText = {
		summary: string;
		previous: string;
		next: string;
	};

	type Props = {
		taskTable: TableInstance<FlowTask>;
		columnCount: number;
		pageSize: number;
		emptyLabel: string;
		pagination: PaginationText;
		openTask: (task: FlowTask) => void;
	};

	let { taskTable, columnCount, pageSize, emptyLabel, pagination, openTask }: Props = $props();
	let rowModel = $derived(taskTable.getRowModel());
	let totalRows = $derived(taskTable.getFilteredRowModel().rows.length);
	let pageCount = $derived(taskTable.getPageCount());
	let pageIndex = $derived(taskTable.getState().pagination.pageIndex);
</script>

<div class="space-y-3">
	<div class="overflow-hidden rounded-lg border bg-card">
		<Table.Root class="min-w-[1080px]">
			<Table.Header class="bg-muted/40">
				{#each taskTable.getHeaderGroups() as headerGroup (headerGroup.id)}
					<Table.Row class="hover:bg-transparent">
						{#each headerGroup.headers as header (header.id)}
							<Table.Head class="h-10">
								{#if !header.isPlaceholder}
									<FlexRender content={header.column.columnDef.header} context={header.getContext()} />
								{/if}
							</Table.Head>
						{/each}
					</Table.Row>
				{/each}
			</Table.Header>
			<Table.Body>
				{#each rowModel.rows as row (row.id)}
					<Table.Row class="cursor-pointer hover:bg-muted/40" onclick={() => openTask(row.original)}>
						{#each row.getVisibleCells() as cell (cell.id)}
							<Table.Cell>
								<FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} />
							</Table.Cell>
						{/each}
					</Table.Row>
				{/each}
				{#if rowModel.rows.length === 0}
					<Table.Row class="hover:bg-transparent">
						<Table.Cell colspan={columnCount} class="py-10 text-center text-muted-foreground">{emptyLabel}</Table.Cell>
					</Table.Row>
				{/if}
			</Table.Body>
		</Table.Root>
	</div>
	{#if totalRows > 0}
		<div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
			<div>
				{pagination.summary
					.replace('{from}', String(totalRows === 0 ? 0 : pageIndex * pageSize + 1))
					.replace('{to}', String(Math.min(totalRows, (pageIndex + 1) * pageSize)))
					.replace('{total}', String(totalRows))}
			</div>
			<div class="flex items-center gap-1">
				<Button variant="outline" size="sm" onclick={() => taskTable.previousPage()} disabled={!taskTable.getCanPreviousPage()}>
					{pagination.previous}
				</Button>
				<span class="px-2 tabular-nums">{pageIndex + 1} / {Math.max(1, pageCount)}</span>
				<Button variant="outline" size="sm" onclick={() => taskTable.nextPage()} disabled={!taskTable.getCanNextPage()}>
					{pagination.next}
				</Button>
			</div>
		</div>
	{/if}
</div>
