<script lang="ts">
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import { FlexRender } from '$lib/components/ui/data-table';
	import * as Table from '$lib/components/ui/table';
	import { cn } from '$lib/utils';
	import type { Table as TableInstance } from '@tanstack/table-core';
	import type { Task } from './task-types';
	import { Button } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { MediaQuery } from 'svelte/reactivity';

	type PaginationText = {
		label: string;
		summary: string;
		previous: string;
		next: string;
	};

	type Props = {
		taskTable: TableInstance<Task>;
		columnCount: number;
		pageSize: number;
		emptyLabel: string;
		pagination: PaginationText;
		openTask: (task: Task) => void;
		focusedTaskID: string;
		mobileLabels: { sort: string; details: string; [key: string]: string | PaginationText };
	};

	let { taskTable, columnCount, pageSize, emptyLabel, pagination, openTask, focusedTaskID, mobileLabels }: Props = $props();
	const isMobile = new MediaQuery('(max-width: 639px)');
	let rowModel = $derived(taskTable.getRowModel());
	let totalRows = $derived(taskTable.getFilteredRowModel().rows.length);
	let pageCount = $derived(taskTable.getPageCount());
	let pageIndex = $derived(taskTable.getState().pagination.pageIndex);
</script>

<div class="space-y-3">
	{#if isMobile.current}
		<Collapsible.Root class="grid gap-2">
			<Collapsible.Trigger>{#snippet child({ props })}<Button {...props} variant="outline" class="justify-self-start">{mobileLabels.sort}</Button>{/snippet}</Collapsible.Trigger>
			<Collapsible.Content class="flex flex-wrap gap-2 rounded-lg border p-2">
				{#each taskTable.getFlatHeaders().filter((header) => header.column.getCanSort()) as header (header.id)}
					<FlexRender content={header.column.columnDef.header} context={header.getContext()} />
				{/each}
			</Collapsible.Content>
		</Collapsible.Root>
		<ul class="grid gap-2">
			{#each rowModel.rows as row (row.id)}
				<li class={cn('min-w-0 rounded-lg border bg-card p-3', row.original.id === focusedTaskID && 'ring-2 ring-primary/30')}>
					<div class="flex min-w-0 items-start gap-2">
						<Button variant="ghost" class="h-auto min-h-11 min-w-0 flex-1 justify-start whitespace-normal px-0 text-left" onclick={() => openTask(row.original)}><span class="line-clamp-3 break-words">{row.original.content}</span></Button>
						{#each row.getVisibleCells().filter((cell) => cell.column.id === 'status') as cell (cell.id)}
							<FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} />
						{/each}
					</div>
					<Collapsible.Root>
						<Collapsible.Trigger>{#snippet child({ props })}<Button {...props} variant="ghost" size="sm" class="px-0 text-muted-foreground">{mobileLabels.details}</Button>{/snippet}</Collapsible.Trigger>
						<Collapsible.Content class="grid gap-3 border-t pt-3 min-[390px]:grid-cols-2">
							{#each row.getVisibleCells().filter((cell) => !['content', 'status'].includes(cell.column.id)) as cell (cell.id)}
								<div class="min-w-0 text-sm"><div class="mb-1 text-xs text-muted-foreground">
									{mobileLabels[cell.column.id] ?? cell.column.id}
								</div><FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} /></div>
							{/each}
						</Collapsible.Content>
					</Collapsible.Root>
				</li>
			{:else}<li class="py-10 text-center text-muted-foreground">{emptyLabel}</li>{/each}
		</ul>
	{:else}
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
					<Table.Row
						class={cn(
							'cursor-pointer hover:bg-muted/40',
							row.original.id === focusedTaskID && 'bg-primary/10 ring-1 ring-primary/30'
						)}
						onclick={() => openTask(row.original)}
					>
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
	{/if}
	<ListPaginationFooter
 onPageChange={(page) => taskTable.setPageIndex(page)}
		totalItems={totalRows}
		{pageIndex}
		{pageSize}
		{pageCount}
		canPreviousPage={taskTable.getCanPreviousPage()}
		canNextPage={taskTable.getCanNextPage()}
		previousPage={() => taskTable.previousPage()}
		nextPage={() => taskTable.nextPage()}
		summary={pagination.summary}
		previousLabel={pagination.previous}
		nextLabel={pagination.next}
		ariaLabel={pagination.label}
	/>
</div>
