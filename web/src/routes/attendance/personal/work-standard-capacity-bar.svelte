<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		actualMinutes: number;
		creditedLeaveMinutes: number;
		targetMinutes: number;
	};

	let {
		actualMinutes,
		creditedLeaveMinutes,
		targetMinutes
	}: Props = $props();
	const text = createPageText(attendanceText);
	const targetPosition = 80;
	const visualCapacityMinutes = $derived(
		Math.max(targetMinutes, 1) / (targetPosition / 100)
	);
	const actualBarMinutes = $derived(
		Math.min(Math.max(actualMinutes, 0), visualCapacityMinutes)
	);
	const leaveBarMinutes = $derived(
		Math.min(
			Math.max(creditedLeaveMinutes, 0),
			Math.max(visualCapacityMinutes - actualBarMinutes, 0)
		)
	);
	const actualWidth = $derived((actualBarMinutes / visualCapacityMinutes) * 100);
	const leaveWidth = $derived((leaveBarMinutes / visualCapacityMinutes) * 100);
	const barLabel = $derived(
		`${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.leave} ${formatWorkStatusDuration(creditedLeaveMinutes, text)}, ${text.workStatus.target} ${formatWorkStatusDuration(targetMinutes, text)}`
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
	<span class="relative block" aria-hidden="true">
		<span
			class="flex h-3.5 overflow-hidden rounded-full bg-muted"
			data-testid="work-standard-progress-track"
		>
			<span
				class="bg-yellow-400"
				style={`width:${actualWidth}%`}
				data-testid="work-standard-actual-segment"
			></span>
			<span class="bg-blue-500" style={`width:${leaveWidth}%`}></span>
		</span>
		<span
			class="absolute -inset-y-1 w-0.5 bg-foreground"
			style={`left:${targetPosition}%`}
			data-testid="work-standard-target-marker"
		></span>
	</span>
</div>

<div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
	<span><i class="mr-1 inline-block size-2 rounded-full bg-yellow-400"></i>{text.workStatus.actual}</span>
	<span><i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>{text.workStatus.leave}</span>
</div>
