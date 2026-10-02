<script lang="ts">
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import type { Column } from '@tanstack/table-core';
	import type { Task } from './task-types';

	type Props = {
		label: string;
		column: Column<Task, unknown> | undefined;
	};

	let { label, column }: Props = $props();
	let sortDirection = $derived(column?.getIsSorted());
	let canSort = $derived(column?.getCanSort());
</script>

<div class="flex items-center gap-1">
	{#if canSort}
		<button
			type="button"
			class="-mx-1 inline-flex min-h-11 items-center gap-1 rounded px-2 py-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground hover:bg-muted hover:text-foreground sm:min-h-0 sm:px-1"
			onclick={() => column?.toggleSorting(sortDirection === 'asc')}
		>
			{label}
			{#if sortDirection === 'asc'}
				<ArrowUpIcon class="size-3" />
			{:else if sortDirection === 'desc'}
				<ArrowDownIcon class="size-3" />
			{:else}
				<ArrowUpDownIcon class="size-3 opacity-40" />
			{/if}
		</button>
	{:else}
		<span class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
	{/if}
</div>
