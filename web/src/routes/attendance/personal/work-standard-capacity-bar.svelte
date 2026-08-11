<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceProgressBar, { type AttendanceProgressSegment } from '../shared/attendance-progress-bar.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { calculateWorkStandardCapacity } from './work-standard-capacity';

	type Props = {
		actualMinutes: number;
		provisionalMinutes: number;
		targetMinutes: number;
		actualSeconds: number;
		provisionalSeconds: number;
		workingCapacitySeconds: number;
		calendarCapacitySeconds: number;
		hasBaseline: boolean;
	};

	let {
		actualMinutes,
		provisionalMinutes,
		targetMinutes,
		actualSeconds,
		provisionalSeconds,
		workingCapacitySeconds,
		calendarCapacitySeconds,
		hasBaseline
	}: Props = $props();
	const text = createPageText(attendanceText);
	const capacity = $derived(
		calculateWorkStandardCapacity({
			actualSeconds,
			provisionalSeconds,
			targetSeconds: targetMinutes * 60,
			workingCapacitySeconds,
			calendarCapacitySeconds,
			hasBaseline
		})
	);
	const progressSegments = $derived<AttendanceProgressSegment[]>([
		{
			id: 'actual',
			widthPercent: capacity.actualWidthPercent,
			className: 'bg-yellow-400',
			testId: 'work-standard-actual-segment'
		}
	]);
	const showsTarget = $derived(capacity.targetPositionPercent !== undefined);
	const barLabel = $derived(
		showsTarget
			? `${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.provisional} ${formatWorkStatusDuration(provisionalMinutes, text)}, ${text.workStatus.target} ${formatWorkStatusDuration(targetMinutes, text)}`
			: `${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.provisional} ${formatWorkStatusDuration(provisionalMinutes, text)}`
	);
</script>

<div
	class={`relative block w-full ${showsTarget ? 'pt-5' : ''}`}
	role="img"
	aria-label={barLabel}
	data-testid="work-standard-capacity-bar"
	data-capacity-stage={capacity.stage}
>
	{#if showsTarget}
		<span
			class="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[9px] font-medium text-foreground"
			style={`left:${capacity.targetPositionPercent ?? 0}%`}
			aria-hidden="true"
		>
			{text.workStatus.target} {formatWorkStatusDuration(targetMinutes, text)}
		</span>
	{/if}
	<span aria-hidden="true">
		<AttendanceProgressBar
			segments={progressSegments}
			trackTestId="work-standard-progress-track"
			targetPosition={capacity.targetPositionPercent}
			targetMarkerTestId={showsTarget ? 'work-standard-target-marker' : undefined}
		/>
	</span>
</div>

<div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
	<span><i class="mr-1 inline-block size-2 rounded-full bg-yellow-400"></i>{text.workStatus.actual}</span>
</div>
