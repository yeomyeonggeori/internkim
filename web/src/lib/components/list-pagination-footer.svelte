<script lang="ts">
	import { Button } from '$lib/components/ui/button';

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
		ariaLabel
	}: Props = $props();

	let fromItem = $derived(totalItems === 0 ? 0 : pageIndex * pageSize + 1);
	let toItem = $derived(Math.min(totalItems, (pageIndex + 1) * pageSize));
	let visiblePageCount = $derived(Math.max(1, pageCount));
	let summaryText = $derived(
		summary
			.replace('{from}', String(fromItem))
			.replace('{to}', String(toItem))
			.replace('{total}', String(totalItems))
	);
</script>

{#if totalItems > 0}
	<nav aria-label={ariaLabel} class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
		<div>{summaryText}</div>
		<div class="flex items-center gap-1">
			<Button variant="outline" size="sm" onclick={previousPage} disabled={!canPreviousPage}>
				{previousLabel}
			</Button>
			<span class="px-2 tabular-nums">{pageIndex + 1} / {visiblePageCount}</span>
			<Button variant="outline" size="sm" onclick={nextPage} disabled={!canNextPage}>
				{nextLabel}
			</Button>
		</div>
	</nav>
{/if}
