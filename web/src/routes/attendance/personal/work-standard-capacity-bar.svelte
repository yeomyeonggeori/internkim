<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';

	type Props = {
		actualMinutes: number;
		creditedLeaveMinutes: number;
		capacityMinutes: number;
		hasBaseline: boolean;
		targetMinutes: number;
	};

	let {
		actualMinutes,
		creditedLeaveMinutes,
		capacityMinutes,
		hasBaseline,
		targetMinutes
	}: Props = $props();
	const text = createPageText(attendanceText);
	const actualBarMinutes = $derived(
		Math.min(Math.max(actualMinutes, 0), capacityMinutes)
	);
	const leaveBarMinutes = $derived(
		Math.min(
			Math.max(creditedLeaveMinutes, 0),
			Math.max(capacityMinutes - actualBarMinutes, 0)
		)
	);
	const unusedMinutes = $derived(
		Math.max(capacityMinutes - actualBarMinutes - leaveBarMinutes, 0)
	);
	const actualWidth = $derived((actualBarMinutes / capacityMinutes) * 100);
	const leaveWidth = $derived((leaveBarMinutes / capacityMinutes) * 100);
	const remainingWidth = $derived((unusedMinutes / capacityMinutes) * 100);
	const targetPosition = $derived(
		hasBaseline
			? (Math.min(targetMinutes, capacityMinutes) / capacityMinutes) * 100
			: 0
	);
	const barLabel = $derived(
		`${text.workStatus.totalCapacity} ${formatWorkStatusDuration(capacityMinutes, text)}, ${text.workStatus.actual} ${formatWorkStatusDuration(actualMinutes, text)}, ${text.workStatus.creditedLeave} ${formatWorkStatusDuration(creditedLeaveMinutes, text)}, ${text.workStatus.remaining} ${formatWorkStatusDuration(unusedMinutes, text)}${hasBaseline ? `, ${text.workStatus.target} ${formatWorkStatusDuration(targetMinutes, text)}` : ''}`
	);
</script>

<div
	class="relative block w-full pt-5"
	role="img"
	aria-label={barLabel}
	data-testid="work-standard-capacity-bar"
>
	{#if hasBaseline}
		<span
			class="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[9px] font-medium text-foreground"
			style={`left:${targetPosition}%`}
			aria-hidden="true"
		>
			{text.workStatus.target} {formatWorkStatusDuration(targetMinutes, text)}
		</span>
	{/if}
	<span class="relative flex h-2.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
		<span class="bg-foreground" style={`width:${actualWidth}%`}></span>
		<span class="bg-blue-500" style={`width:${leaveWidth}%`}></span>
		<span class="bg-muted-foreground/20" style={`width:${remainingWidth}%`}></span>
		{#if hasBaseline}
			<span
				class="absolute inset-y-0 w-0.5 bg-foreground"
				style={`left:${targetPosition}%`}
				data-testid="work-standard-target-marker"
			></span>
		{/if}
	</span>
	<span
		class="mt-1 flex justify-between text-[9px] text-muted-foreground tabular-nums"
		aria-hidden="true"
	>
		<span>{formatWorkStatusDuration(0, text)}</span>
		<span>{formatWorkStatusDuration(capacityMinutes, text)}</span>
	</span>
</div>

<div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
	<span><i class="mr-1 inline-block size-2 rounded-full bg-foreground"></i>{text.workStatus.actual}</span>
	<span><i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>{text.workStatus.creditedLeave}</span>
	<span><i class="mr-1 inline-block size-2 rounded-full bg-muted-foreground/20"></i>{text.workStatus.remaining}</span>
</div>
