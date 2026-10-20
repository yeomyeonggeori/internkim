<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { label, title }: { label: string; title: string } = $props();
</script>

<div role="status" aria-busy="true" data-task-content-skeleton="relationships">
	<span class="sr-only">{label}</span>
	<div aria-hidden="true" class="space-y-3">
		<h3 class="text-sm font-semibold text-foreground">{title}</h3>
		{#each [0, 1, 2] as row (row)}
			<div class="flex min-h-12 items-center gap-3 rounded-lg border p-3"><Skeleton class="size-5 shrink-0" /><Skeleton class="h-4 flex-1" /><Skeleton class="h-5 w-16" /></div>
		{/each}
	</div>
</div>
