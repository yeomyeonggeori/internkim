<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	let { label, compact = false, action = false }: { label: string; compact?: boolean; action?: boolean } = $props();
</script>

<div role="status" aria-busy="true" data-messenger-list-skeleton>
	<span class="sr-only">{label}</span>
	<div aria-hidden="true" class={compact ? 'space-y-1' : 'space-y-2'}>
		{#each [0, 1, 2, 3, 4] as row}
			<div class={compact ? 'flex h-8 items-center gap-2 px-2' : 'flex items-center gap-3 px-2 py-2'}>
				<Skeleton class={compact ? 'size-4 shrink-0 rounded-full' : 'size-8 shrink-0 rounded-full'} />
				<Skeleton class={row % 2 ? 'h-4 w-24 max-w-full' : 'h-4 w-32 max-w-full'} />
				{#if action}<Skeleton class="ml-auto h-8 w-14 shrink-0" />{/if}
			</div>
		{/each}
	</div>
</div>
