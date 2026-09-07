<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';

	type Props = {
		label: string;
		color?: string;
		showMarker?: boolean;
		class?: string;
	};

	let { label, color = '', showMarker = true, class: className }: Props = $props();
</script>

<Badge
	variant="outline"
	class={cn(
		'h-5 gap-1.5 border-border/70 bg-muted/30 px-1.5 py-0 text-[11px] font-medium shadow-none',
		color
			? 'text-[var(--marker-color)] dark:text-[color-mix(in_oklch,var(--marker-color)_65%,white)]'
			: 'text-muted-foreground',
		className
	)}
	style={color ? `--marker-color: ${color}` : undefined}
>
	{#if showMarker}
		<ColorMarker {color} />
	{/if}
	{label}
</Badge>
