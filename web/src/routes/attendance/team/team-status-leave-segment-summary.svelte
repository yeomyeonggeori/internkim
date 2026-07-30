<script lang="ts">
	import TimeRangeText from '$lib/components/time-range-text.svelte';
	import DurationText from '../shared/duration-text.svelte';

	type Props = {
		label: string;
		startTime: string;
		endTime: string;
		durationMinutes: number;
	};

	let { label, startTime, endTime, durationMinutes }: Props = $props();
</script>

<div class="grid min-w-0 gap-0.5" data-slot="leave-segment-summary">
	<div class="flex min-w-0 items-center justify-between gap-3">
		<div class="flex min-w-0 items-center gap-1.5">
			<span
				class="h-3 w-1 shrink-0 rounded-full bg-info"
				aria-hidden="true"
				data-slot="leave-segment-marker"
			></span>
			<span class="min-w-0 truncate text-xs font-medium">{label}</span>
		</div>
		<DurationText minutes={durationMinutes} size="small" tone="default" />
	</div>
	<TimeRangeText {startTime} {endTime} size="extraSmall" tone="muted" />
</div>
