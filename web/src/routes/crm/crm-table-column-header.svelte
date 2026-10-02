<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import type { CRMSortState } from './crm-table-sort';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { crmText } from './text';
	const text = createPageText(crmText);

	type Props = {
		label: string;
		sortKey: string;
		sort: CRMSortState | null;
		onSort: (key: string) => void;
	};

	let { label, sortKey, sort, onSort }: Props = $props();

	const activeDirection = $derived(sort?.key === sortKey ? sort.direction : null);
	const SortIcon = $derived(
		activeDirection === 'ascending'
			? ArrowUpIcon
			: activeDirection === 'descending'
				? ArrowDownIcon
				: ArrowUpDownIcon
	);
</script>

<Button type="button" variant="ghost" size="sm" class="-ml-2 h-8 gap-1.5 px-2 font-medium" onclick={() => onSort(sortKey)}>
	{label}
	<SortIcon class="size-3.5 shrink-0 opacity-60" aria-hidden="true" />
	{#if activeDirection}<span class="sr-only">{activeDirection === 'ascending' ? text.sortAscending : text.sortDescending}</span>{/if}
</Button>
