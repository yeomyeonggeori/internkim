<script lang="ts">
	import { cn } from '$lib/utils';

	let { importance, label, class: className }: { importance: number; label: string; class?: string } = $props();
	const barHeights = ['h-1', 'h-1.5', 'h-2', 'h-2.5', 'h-3'];
</script>

<span role="img" aria-label={label} title={label} class={cn('inline-flex h-3 shrink-0 items-end gap-0.5', className)}>
	{#each barHeights as barHeight, index (barHeight)}
		<span class={cn('w-0.75 rounded-[1px]', barHeight, index < importance ? 'bg-foreground' : 'bg-border')}></span>
	{/each}
</span>
