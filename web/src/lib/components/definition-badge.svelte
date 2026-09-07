<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';

	type Props = {
		label: string;
		color?: string;
		class?: string;
	};

	let { label, color = '', class: className }: Props = $props();
</script>

<Badge
	variant="outline"
	class={cn(
		'h-5 gap-1.5 px-1.5 py-0 text-[11px] font-medium shadow-none',
		color
			? 'border-[var(--marker-color)] bg-[color-mix(in_oklch,var(--marker-color)_8%,transparent)] text-[var(--marker-color)] dark:bg-[color-mix(in_oklch,var(--marker-color)_12%,transparent)] dark:text-[color-mix(in_oklch,var(--marker-color)_65%,white)]'
			: 'border-border/70 bg-muted/30 text-muted-foreground',
		className
	)}
	style={color ? `--marker-color: ${color}` : undefined}
>
	{label}
</Badge>
