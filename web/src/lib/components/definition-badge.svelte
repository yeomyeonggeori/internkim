<script lang="ts">
	import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
	import { cn } from '$lib/utils';

	type Props = {
		label: string;
		color?: string;
		class?: string;
	};

	let { label, color = '', class: className }: Props = $props();
</script>

<ColoredOutlineBadge
	{color}
	class={cn(
		'h-5 gap-1.5 px-1.5 py-0 text-[11px] font-medium shadow-none',
		color
			? 'bg-[color-mix(in_oklch,var(--badge-color)_8%,transparent)] dark:bg-[color-mix(in_oklch,var(--badge-color)_12%,transparent)]'
			: 'border-border/70 bg-muted/30 text-muted-foreground',
		className
	)}
>
	{label}
</ColoredOutlineBadge>
