<script lang="ts">
	export type AttendanceProgressSegment = {
		id: string;
		widthPercent: number;
		className?: string;
		color?: string;
		testId?: string;
	};

	type Props = {
		segments: AttendanceProgressSegment[];
		trackTestId?: string;
		targetPosition?: number;
		targetMarkerTestId?: string;
	};

	let {
		segments,
		trackTestId,
		targetPosition,
		targetMarkerTestId
	}: Props = $props();

	function boundedPercent(value: number): number {
		return Math.min(100, Math.max(0, value));
	}
</script>

<span
	class="relative block h-1.5 w-full overflow-visible rounded-full bg-muted"
	data-attendance-progress-bar
	data-testid={trackTestId}
>
	<span class="flex h-full overflow-hidden rounded-full">
		{#each segments as segment (segment.id)}
			<span
				class={`h-full shrink-0 ${segment.className ?? ''}`}
				style:width={`${boundedPercent(segment.widthPercent)}%`}
				style:background-color={segment.color}
				data-testid={segment.testId}
			></span>
		{/each}
	</span>
	{#if targetPosition !== undefined}
		<span
			class="absolute -inset-y-1 w-0.5 bg-foreground"
			style:left={`${boundedPercent(targetPosition)}%`}
			data-testid={targetMarkerTestId}
		></span>
	{/if}
</span>
