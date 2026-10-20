<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import HashIcon from '@lucide/svelte/icons/hash';
	let { label, compact = false, actionLabel = '' }: { label: string; compact?: boolean; actionLabel?: string } = $props();
</script>

<div role="status" aria-busy="true" data-messenger-list-skeleton>
	<span class="sr-only">{label}</span>
	<div aria-hidden="true" class={compact ? 'space-y-1' : 'space-y-2'}>
		{#each [0, 1, 2, 3, 4] as row}
			<div class={compact ? 'flex h-8 items-center gap-2 px-2' : 'flex items-center gap-3 px-2 py-2'}>
				{#if actionLabel}
					<HashIcon class="text-muted-foreground size-4 shrink-0" />
				{:else}
					<Skeleton class={compact ? 'size-4 shrink-0 rounded-full' : 'size-8 shrink-0 rounded-full'} />
				{/if}
				<Skeleton class={row % 2 ? 'h-4 w-24 max-w-full' : 'h-4 w-32 max-w-full'} />
				{#if actionLabel}<Button class="ml-auto" size="sm" variant="outline" disabled>{actionLabel}</Button>{/if}
			</div>
		{/each}
	</div>
</div>
