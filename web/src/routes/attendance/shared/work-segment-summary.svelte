<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import TimeRangeText from '$lib/components/time-range-text.svelte';
	import DurationText from './duration-text.svelte';

	type Props = {
		locationName: string;
		locationColor?: string;
		startTime: string;
		endTime: string;
		durationMinutes?: number;
		isOpen: boolean;
	};

	let {
		locationName,
		locationColor,
		startTime,
		endTime,
		durationMinutes,
		isOpen
	}: Props = $props();
</script>

<div class="grid min-w-0 gap-0.5" data-slot="work-segment-summary">
	<div class="flex min-w-0 items-center justify-between gap-3">
		<div class="flex min-w-0 items-center gap-1.5">
			<ColorMarker color={locationColor} data-slot="work-segment-marker" />
			<span class="min-w-0 truncate text-xs font-medium">{locationName}</span>
		</div>
		<DurationText
			minutes={durationMinutes ?? 0}
			showZero={isOpen}
			size="small"
			tone={isOpen ? 'info' : 'default'}
		/>
	</div>
	<TimeRangeText
		{startTime}
		{endTime}
		size="extraSmall"
		tone="muted"
		endTone={isOpen ? 'info' : 'inherit'}
	/>
</div>
