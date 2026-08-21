<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import FilterIcon from '@lucide/svelte/icons/sliders-horizontal';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		activeCount: number;
		onReset: () => void;
		children: Snippet;
	};

	let { text, activeCount, onReset, children }: Props = $props();

	const buttonLabel = $derived(
		activeCount > 0 ? text.filterCount.replace('{count}', String(activeCount)) : text.filterButton
	);
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" variant="outline" size="sm" class="gap-2" aria-label={text.filterButton}>
				<FilterIcon class="size-4" />
				{buttonLabel}
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content
		align="start"
		sideOffset={8}
		class="w-[min(18rem,calc(100vw-2rem))] space-y-4 p-3"
		data-crm-filter-panel
	>
		<div class="grid gap-2">
			{@render children()}
		</div>
		<div class="flex items-center justify-end border-t pt-3">
			<Button type="button" variant="ghost" size="sm" onclick={onReset}>
				<RotateCcwIcon class="size-4" />
				{text.resetFilters}
			</Button>
		</div>
	</DropdownMenu.Content>
</DropdownMenu.Root>
