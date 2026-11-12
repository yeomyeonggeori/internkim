<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { cn } from '$lib/utils';
	import { taskRoadmapColumnsClass } from './task-roadmap-layout';

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
	<div class={cn(taskRoadmapColumnsClass, 'h-12 border-b')}>
		<div class="border-r max-sm:hidden"></div>
		<div class="grid place-items-center"><Skeleton class="size-7 rounded-md" /></div>
		<div class="flex items-center justify-around">
			{#each { length: 14 }, index (index)}<Skeleton class={cn('h-6 w-4', index >= 7 && 'max-sm:hidden')} />{/each}
		</div>
		<div class="grid place-items-center"><Skeleton class="size-7 rounded-md" /></div>
	</div>
	{#each barRows as bar, index (index)}
		<div class={cn(taskRoadmapColumnsClass, 'border-b last:border-b-0')}>
			<div class="flex h-12 min-w-0 items-center gap-2.5 border-r px-3 max-sm:col-span-3 max-sm:h-10 max-sm:border-r-0">
				<Skeleton class="size-6 rounded-full" />
				<Skeleton class="h-4 flex-1" />
			</div>
			<div></div>
			<div class="relative h-12 max-sm:h-9">
				<Skeleton class="absolute inset-y-0 my-auto h-7" style={`left: ${bar.left}%; width: ${bar.width}%`} />
			</div>
			<div></div>
		</div>
	{/each}
</div>
