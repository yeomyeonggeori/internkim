<script lang="ts">
	import * as Item from '$lib/components/ui/item/index.js';
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { label }: { label: string } = $props();

	const rows = [
		{ name: 'w-20', reason: 'w-28', text: 'w-4/5' },
		{ name: 'w-16', reason: 'w-24', text: 'w-3/5' },
		{ name: 'w-24', reason: 'w-20', text: 'w-2/3' },
		{ name: 'w-14', reason: 'w-28', text: 'w-1/2' },
		{ name: 'w-20', reason: 'w-24', text: 'w-3/4' }
	];
</script>

<div role="status" aria-busy="true" class="min-h-0 flex-1 overflow-hidden">
	<span class="sr-only">{label}</span>
	<Item.Group aria-hidden="true" class="gap-1 p-2">
		{#each rows as row, index (index)}
			<Item.Root size="sm" data-inbox-skeleton-row>
				<Item.Media>
					<Skeleton class="size-8 rounded-full" />
				</Item.Media>
				<Item.Content class="min-w-0">
					<Item.Title class="h-[1lh] w-full">
						<Skeleton class="h-3.5 {row.name}" />
						<Skeleton class="ml-auto h-3 w-8" />
					</Item.Title>
					<div class="flex h-4 items-center"><Skeleton class="h-3 {row.reason}" /></div>
					<Item.Description class="flex h-[1lh] items-center"><Skeleton class="h-3.5 {row.text}" /></Item.Description>
				</Item.Content>
			</Item.Root>
		{/each}
	</Item.Group>
</div>
