<script lang="ts">
	export type SegmentTooltipRow = {
		id: string;
		color?: string;
		locationName: string;
		timeLabel: string;
		durationLabel?: string;
	};

	let { rows }: { rows: SegmentTooltipRow[] } = $props();
</script>

<div class="grid grid-cols-[max-content_max-content_max-content] items-center gap-x-3 gap-y-1 text-xs">
	{#each rows as row (row.id)}
		<div class="flex items-center gap-1.5">
			<span
				class="size-2 shrink-0 rounded-[2px]"
				style:background-color={row.color ?? 'var(--color-muted-foreground)'}
			></span>
			<span class="whitespace-nowrap text-muted-foreground">{row.locationName}</span>
		</div>
		<span class="whitespace-nowrap tabular-nums text-muted-foreground">{row.timeLabel}</span>
		<span class="whitespace-nowrap text-right font-medium tabular-nums text-foreground">{row.durationLabel ?? ''}</span>
	{/each}
</div>
