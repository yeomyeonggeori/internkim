<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceProgressBar, { type AttendanceProgressSegment } from '../shared/attendance-progress-bar.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		actualMinutes: number;
		provisionalMinutes: number;
		targetMinutes: number;
	};

	let {
		actualMinutes,
		provisionalMinutes,
		targetMinutes
	}: Props = $props();
	const text = createPageText(attendanceText);
	const targetPosition = 80;
	const visualCapacityMinutes = $derived(
		Math.max(targetMinutes, 1) / (targetPosition / 100)
	);
	const actualBarMinutes = $derived(
		Math.min(
			Math.max(actualMinutes, 0) + Math.max(provisionalMinutes, 0),
			visualCapacityMinutes
		)
	);
	const actualWidth = $derived((actualBarMinutes / visualCapacityMinutes) * 100);
	const progressSegments = $derived<AttendanceProgressSegment[]>([
		{
			id: 'actual',
			widthPercent: actualWidth,
			className: 'bg-yellow-400',
			testId: 'work-standard-actual-segment'
		}
	]);
	const barLabel = $derived(
		`${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.provisional} ${formatWorkStatusDuration(provisionalMinutes, text)}, ${text.workStatus.target} ${formatWorkStatusDuration(targetMinutes, text)}`
	);
</script>

<div
	class="relative block w-full pt-5"
	role="img"
	aria-label={barLabel}
	data-testid="work-standard-capacity-bar"
>
	<span
		class="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[9px] font-medium text-foreground"
		style={`left:${targetPosition}%`}
		aria-hidden="true"
	>
		{text.workStatus.target} {formatWorkStatusDuration(targetMinutes, text)}
	</span>
	<span aria-hidden="true">
		<AttendanceProgressBar
			segments={progressSegments}
			trackTestId="work-standard-progress-track"
			{targetPosition}
			targetMarkerTestId="work-standard-target-marker"
		/>
	</span>
</div>

<div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
	<span><i class="mr-1 inline-block size-2 rounded-full bg-yellow-400"></i>{text.workStatus.actual}</span>
</div>
