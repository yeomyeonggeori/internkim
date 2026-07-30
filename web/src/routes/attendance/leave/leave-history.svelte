<script lang="ts">
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import { Button } from '$lib/components/ui/button';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveRequest } from './employee-leave-types';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import LeaveHistoryRow from './leave-history-row.svelte';
	import {
		buildLeaveHistory,
		filterLeaveHistory,
		type LeaveHistoryFilter
	} from './leave-history-model';

	type Props = {
		text: AttendanceText['leave'];
		onEdit: (request: EmployeeLeaveRequest) => void;
		onResubmit: (request: EmployeeLeaveRequest) => void;
	};

	let { text, onEdit, onResubmit }: Props = $props();
	const employeeLeave = getEmployeeLeaveState();
	const pageSize = 3;
	let filter = $state<LeaveHistoryFilter>('requests');
	let pageIndex = $state(0);
	const history = $derived(
		buildLeaveHistory(
			employeeLeave.payload?.requests ?? [],
			employeeLeave.payload?.ledgerEntries ?? [],
			employeeLeave.payload?.leaveTypes ?? []
		)
	);
	const visibleHistory = $derived(filterLeaveHistory(history, filter));
	const pageCount = $derived(Math.ceil(visibleHistory.length / pageSize));
	const currentPageIndex = $derived(Math.min(pageIndex, Math.max(0, pageCount - 1)));
	const paginatedHistory = $derived(
		visibleHistory.slice(
			currentPageIndex * pageSize,
			(currentPageIndex + 1) * pageSize
		)
	);

	function selectFilter(nextFilter: LeaveHistoryFilter): void {
		filter = nextFilter;
		pageIndex = 0;
	}

	async function cancelRequest(request: EmployeeLeaveRequest): Promise<void> {
		if (!request.canCancel || employeeLeave.isMutating) return;
		try {
			await employeeLeave.cancel(request.id);
		} catch {
			return;
		}
	}
</script>

<div class="grid gap-4 px-4 pt-[3px] pb-5 sm:px-6" data-testid="leave-history">
	<div
		class="flex max-w-full gap-1 overflow-x-auto"
		role="group"
		aria-label={text.historyTab}
	>
		<Button
			type="button"
			size="sm"
			variant={filter === 'requests' ? 'secondary' : 'ghost'}
			aria-pressed={filter === 'requests'}
			onclick={() => selectFilter('requests')}
		>
			{text.historyRequests}
		</Button>
		<Button
			type="button"
			size="sm"
			variant={filter === 'balance' ? 'secondary' : 'ghost'}
			aria-pressed={filter === 'balance'}
			onclick={() => selectFilter('balance')}
		>
			{text.historyBalance}
		</Button>
	</div>

	{#if employeeLeave.mutationErrorMessage}
		<p class="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
			{employeeLeave.mutationErrorMessage}
		</p>
	{/if}

	{#if paginatedHistory.length}
		<div class="divide-y" data-testid="leave-history-list">
			{#each paginatedHistory as item (item.id)}
				<LeaveHistoryRow
					{item}
					{text}
					isMutating={employeeLeave.isMutating}
					onCancel={(request) => void cancelRequest(request)}
					{onEdit}
					{onResubmit}
				/>
			{/each}
		</div>
	{:else}
		<div class="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
			{text.historyEmpty}
		</div>
	{/if}

	<ListPaginationFooter
		totalItems={visibleHistory.length}
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
	/>
</div>
