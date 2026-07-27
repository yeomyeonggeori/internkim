<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import type { Snippet } from 'svelte';

	let {
		count,
		label,
		icon
	}: {
		count: number;
		label?: string;
		icon?: Snippet;
	} = $props();
</script>

<Badge variant="outline" class="h-5 min-w-5 shrink-0 justify-center gap-1 rounded-full px-1.5 font-mono tabular-nums" aria-label={label}>
	{@render icon?.()}
	{count}
</Badge>
