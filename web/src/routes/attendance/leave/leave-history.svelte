<script lang="ts">
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveRequest } from './employee-leave-types';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import LeaveHistoryRow from './leave-history-row.svelte';
	import { buildLeaveHistory } from './leave-history-model';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';

	type Props = {
		text: AttendanceText['leave'];
	};

	let { text }: Props = $props();
	const employeeLeave = getEmployeeLeaveState();
	const pageSize = 3;
	let pageIndex = $state(0);
	const history = $derived(buildLeaveHistory(employeeLeave.payload?.requests ?? []));
	const pageCount = $derived(Math.ceil(history.length / pageSize));
	const currentPageIndex = $derived(Math.min(pageIndex, Math.max(0, pageCount - 1)));
	const paginatedHistory = $derived(
		history.slice(
			currentPageIndex * pageSize,
			(currentPageIndex + 1) * pageSize
		)
	);

	async function cancelRequest(request: EmployeeLeaveRequest): Promise<void> {
		if (!request.canCancel || employeeLeave.isMutating) return;
		try {
			await employeeLeave.cancel(request.id);
		} catch {
			return;
		}
	}
</script>

<div class="grid min-w-0 grid-cols-1 gap-4 px-4 pt-4 pb-5 sm:px-6" data-testid="leave-history">
	{#if employeeLeave.mutationErrorMessage}
		<p class="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
			{employeeLeave.mutationErrorMessage}
		</p>
	{/if}

	{#if !employeeLeave.payload && !employeeLeave.errorMessage}
		<AttendanceLoadingSkeleton kind="records" rowCount={3} />
	{:else if paginatedHistory.length}
		<div class="divide-y" data-testid="leave-history-list">
			{#each paginatedHistory as item (item.id)}
				<LeaveHistoryRow
					{item}
					{text}
					isMutating={employeeLeave.isMutating}
					onCancel={(request) => void cancelRequest(request)}
				/>
			{/each}
		</div>
	{:else}
		<div class="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
			{text.historyEmpty}
		</div>
	{/if}

	{#if employeeLeave.payload}<ListPaginationFooter
 onPageChange={(page) => (pageIndex = page)}
		totalItems={history.length}
		pageIndex={currentPageIndex}
		{pageSize}
		{pageCount}
		canPreviousPage={currentPageIndex > 0}
		canNextPage={currentPageIndex + 1 < pageCount}
		previousPage={() => (pageIndex = Math.max(0, currentPageIndex - 1))}
		nextPage={() => (pageIndex = Math.min(pageCount - 1, currentPageIndex + 1))}
		summary={text.historyPaginationSummary}
		previousLabel={text.historyPaginationPrevious}
		nextLabel={text.historyPaginationNext}
		ariaLabel={text.historyPaginationLabel}
	/>{/if}
</div>
