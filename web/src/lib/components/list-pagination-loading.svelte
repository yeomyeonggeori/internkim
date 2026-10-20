<script lang="ts">
	import * as Pagination from '$lib/components/ui/pagination';
	import { Skeleton } from '$lib/components/ui/skeleton';

	type Props = {
		previousLabel: string;
		nextLabel: string;
		showSummary?: boolean;
	};

	let { previousLabel, nextLabel, showSummary = true }: Props = $props();
</script>

<div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
	{#if showSummary}<Skeleton class="h-4 w-24" />{/if}
	<Pagination.Root class="mx-0 w-auto" count={1} perPage={1}>
		<Pagination.Content>
			<Pagination.Item>
				<Pagination.Previous label={previousLabel} aria-label={previousLabel} disabled />
			</Pagination.Item>
			<Pagination.Item><Skeleton class="h-11 w-8 sm:h-8" /></Pagination.Item>
			<Pagination.Item>
				<Pagination.Next label={nextLabel} aria-label={nextLabel} disabled />
			</Pagination.Item>
		</Pagination.Content>
	</Pagination.Root>
</div>
