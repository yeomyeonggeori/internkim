<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { label }: { label: string } = $props();

	const barRows = [
		{ left: 8, width: 30 },
		{ left: 20, width: 18 },
		{ left: 4, width: 40 },
		{ left: 32, width: 22 },
		{ left: 12, width: 26 },
		{ left: 44, width: 16 }
	];
</script>

<div class="overflow-hidden rounded-lg border" role="status" aria-label={label} data-task-roadmap-skeleton>
	<div class="flex h-14 border-b">
		<div class="w-80 shrink-0 border-r max-sm:w-56"></div>
		<div class="flex flex-1 items-end gap-3 px-3 pb-2">
			{#each { length: 20 }, index (index)}<Skeleton class="h-4 w-5" />{/each}
		</div>
	</div>
	{#each barRows as bar, index (index)}
		<div class="flex h-12 border-b last:border-b-0">
			<div class="flex w-80 shrink-0 items-center gap-2.5 border-r pr-3 pl-9 max-sm:w-56 max-sm:pl-4">
				<Skeleton class="size-6 rounded-full" />
				<Skeleton class="h-4 flex-1" />
			</div>
			<div class="relative flex-1">
				<Skeleton class="absolute top-1/2 -mt-3.5 h-7" style={`left: ${bar.left}%; width: ${bar.width}%`} />
			</div>
		</div>
	{/each}
</div>
