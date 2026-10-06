<script lang="ts">
	import * as Pagination from '$lib/components/ui/pagination';
	import { MediaQuery } from 'svelte/reactivity';
	const mobile = new MediaQuery('(max-width: 639px)');

	type Props = {
		totalItems: number;
		pageIndex: number;
		pageSize: number;
		pageCount: number;
		canPreviousPage: boolean;
		canNextPage: boolean;
		previousPage: () => void;
		nextPage: () => void;
		summary: string;
		previousLabel: string;
		nextLabel: string;
		ariaLabel?: string;
		onPageChange: (pageIndex: number) => void;
		disabled?: boolean;
		showSummary?: boolean;
	};

	let {
		totalItems,
		pageIndex,
		pageSize,
		pageCount,
		canPreviousPage,
		canNextPage,
		previousPage,
		nextPage,
		summary,
		previousLabel,
		nextLabel,
		ariaLabel,
		onPageChange,
		disabled = false,
		showSummary = true
	}: Props = $props();

	let fromItem = $derived(totalItems === 0 ? 0 : pageIndex * pageSize + 1);
	let toItem = $derived(Math.min(totalItems, (pageIndex + 1) * pageSize));
	let summaryText = $derived(
		summary
			.replace('{from}', String(fromItem))
			.replace('{to}', String(toItem))
			.replace('{total}', String(totalItems))
	);
</script>

{#if totalItems > 0}
 <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
  {#if showSummary}<div>{summaryText}</div>{/if}
  <Pagination.Root class="mx-0 w-auto" aria-label={ariaLabel} count={totalItems} perPage={pageSize} page={pageIndex + 1} siblingCount={mobile.current ? 0 : 1} onPageChange={(page) => {if (!disabled && page !== pageIndex + 1 && page >= 1 && page <= pageCount) onPageChange(page - 1);}}>
   {#snippet children({ pages, currentPage })}
    <Pagination.Content>
     <Pagination.Item><Pagination.Previous label={previousLabel} aria-label={previousLabel} disabled={disabled || !canPreviousPage} /></Pagination.Item>
     {#each pages as page (page.key)}
      {#if page.type === 'ellipsis'}<Pagination.Item><Pagination.Ellipsis /></Pagination.Item>
      {:else}<Pagination.Item><Pagination.Link {page} isActive={currentPage === page.value} disabled={disabled} /></Pagination.Item>{/if}
     {/each}
     <Pagination.Item><Pagination.Next label={nextLabel} aria-label={nextLabel} disabled={disabled || !canNextPage} /></Pagination.Item>
    </Pagination.Content>
   {/snippet}
  </Pagination.Root>
 </div>
{/if}
