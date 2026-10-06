<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	let { label }: { label: string } = $props();
</script>

<div role="status" aria-busy="true" class="h-full min-h-0 flex-1 overflow-hidden" data-message-skeleton>
	<span class="sr-only">{label}</span>
	<div aria-hidden="true" class="flex h-full flex-col gap-8 px-4 pt-12 pb-[calc(var(--dock-height,0px)+1rem)]">
		{#each [0, 1, 2, 3, 4, 5] as index}
			<div class="flex gap-3" class:flex-row-reverse={index % 3 === 0}>
				<Skeleton class="size-9 shrink-0 rounded-full" />
				<div class="flex max-w-[70%] flex-col gap-2">
					<Skeleton class="h-4 w-24" />
					<Skeleton class="h-16 w-64 max-w-full rounded-2xl" />
				</div>
			</div>
		{/each}
	</div>
</div>
