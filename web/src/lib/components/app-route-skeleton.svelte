<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="flex min-w-0 flex-1 flex-col gap-4 p-3 md:p-6" data-testid="app-route-skeleton" aria-hidden="true">
	<div class="flex flex-wrap items-center gap-2">
		<Skeleton class="h-9 w-44" />
		<Skeleton class="h-9 w-32" />
		<Skeleton class="ml-auto h-9 w-28" />
	</div>
	<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		{#each Array.from({ length: 6 }) as _, cardIndex (cardIndex)}
			<Skeleton class="h-28 w-full rounded-xl" />
		{/each}
	</div>
	<Skeleton class="h-64 w-full rounded-xl" />
</div>
