<script lang="ts">
	import { Badge, type BadgeVariant } from '$lib/components/ui/badge';
	import type { Snippet } from 'svelte';

	let {
		count,
		label,
		icon,
		variant = 'outline'
	}: {
		count: number;
		label?: string;
		icon?: Snippet;
		variant?: BadgeVariant;
	} = $props();
</script>

<Badge {variant} class="h-5 min-w-5 shrink-0 justify-center gap-1 px-1.5 font-mono tabular-nums sm:rounded-full" aria-label={label}>
	{@render icon?.()}
	{count}
</Badge>
