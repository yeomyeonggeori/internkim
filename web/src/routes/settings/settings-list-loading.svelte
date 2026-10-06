<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	let { label }: { label: string } = $props();
</script>

<div role="status" aria-label={label} aria-busy="true" class="grid gap-2">
	{#each [0, 1] as row (row)}<div aria-hidden="true" class="flex items-center justify-between gap-4 rounded-md border px-4 py-3"><div class="grid min-w-0 flex-1 gap-2"><Skeleton class="h-4 w-2/3 max-w-40" /><Skeleton class="h-3 w-full max-w-64" /></div><Skeleton class="h-8 w-16 shrink-0" /></div>{/each}
</div>
