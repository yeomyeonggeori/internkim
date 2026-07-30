<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	type Props = {
		rowCount?: number;
	};

	let { rowCount = 6 }: Props = $props();
</script>

<div class="space-y-4" data-testid="attendance-skeleton" aria-hidden="true">
	<div class="flex flex-wrap items-center gap-2">
		<Skeleton class="h-9 w-40" />
		<Skeleton class="h-9 w-48" />
		<Skeleton class="ml-auto h-9 w-32" />
	</div>
	<div class="space-y-2 rounded-xl border p-4">
		{#each Array.from({ length: rowCount }) as _, rowIndex (rowIndex)}
			<div class="flex items-center gap-3">
				<Skeleton class="size-8 rounded-full" />
				<Skeleton class="h-4 flex-1" />
				<Skeleton class="h-4 w-16" />
			</div>
		{/each}
	</div>
</div>
