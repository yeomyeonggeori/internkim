<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import {
		calculatePeriodCapacityMinutes,
		formatWorkStatusDuration
	} from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const defaultCapacityMinutes = 24 * 60;
	const capacityMinutes = $derived(
		status
			? calculatePeriodCapacityMinutes(status.periodStart, status.periodEnd)
			: defaultCapacityMinutes
	);
	const actualBarMinutes = $derived(
		status ? Math.min(Math.max(status.actualMinutes, 0), capacityMinutes) : 0
	);
	const leaveBarMinutes = $derived(
		status
			? Math.min(
					Math.max(status.creditedLeaveMinutes, 0),
					Math.max(capacityMinutes - actualBarMinutes, 0)
				)
			: 0
	);
	const unusedMinutes = $derived(
		Math.max(capacityMinutes - actualBarMinutes - leaveBarMinutes, 0)
	);
	const actualWidth = $derived(
		(actualBarMinutes / capacityMinutes) * 100
	);
	const leaveWidth = $derived(
		(leaveBarMinutes / capacityMinutes) * 100
	);
	const remainingWidth = $derived(
		(unusedMinutes / capacityMinutes) * 100
	);
	const targetPosition = $derived(
		status?.hasBaseline
			? (Math.min(status.targetMinutes, capacityMinutes) / capacityMinutes) * 100
			: 0
	);
</script>

<div class="mt-3 border-t pt-3" data-testid="personal-work-standard">
	<div class="mb-2 flex items-center justify-between gap-2">
		<p class="text-[11px] font-medium">{text.workStatus.standard}</p>
		{#if workStatus.isLoading}
			<span class="text-[10px] text-muted-foreground">{text.loading}</span>
		{:else if status}
			<span class="text-xs font-medium">{text.workStatus[status.workMode]}</span>
		{/if}
	</div>

	{#if workStatus.errorMessage}
		<p class="text-[10px] text-destructive">{text.workStatus.loadFailed}</p>
	{:else if status}
		<div class="mb-3">
			<p class="text-[10px] text-muted-foreground">
				{status.periodStart === status.periodEnd
					? status.periodStart
					: `${status.periodStart}–${status.periodEnd}`}
			</p>
			{#if status.hasBaseline}
				<div class="mt-1 tabular-nums">
					<p class="text-lg font-semibold">{formatWorkStatusDuration(status.fulfilledMinutes, text)}</p>
					<p class="text-[10px] text-muted-foreground">
						/ {text.workStatus.target} {formatWorkStatusDuration(status.targetMinutes, text)}
					</p>
				</div>
			{:else}
				<p class="mt-1 text-lg font-semibold text-muted-foreground">{text.workStatus.noBaseline}</p>
			{/if}
		</div>

		<div
			class="relative pt-5"
			data-testid="work-standard-capacity-bar"
			aria-label={`${text.workStatus.totalCapacity} ${formatWorkStatusDuration(capacityMinutes, text)}, ${text.workStatus.actual} ${formatWorkStatusDuration(status.actualMinutes, text)}, ${text.workStatus.creditedLeave} ${formatWorkStatusDuration(status.creditedLeaveMinutes, text)}, ${text.workStatus.remaining} ${formatWorkStatusDuration(unusedMinutes, text)}`}
		>
			{#if status.hasBaseline}
				<div
					class="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[9px] font-medium text-foreground"
					style={`left:${targetPosition}%`}
				>
					{text.workStatus.target} {formatWorkStatusDuration(status.targetMinutes, text)}
				</div>
			{/if}
			<div
				class="relative flex h-2.5 overflow-hidden rounded-full bg-muted"
			>
				<div class="bg-foreground" style={`width:${actualWidth}%`}></div>
				<div class="bg-blue-500" style={`width:${leaveWidth}%`}></div>
				<div class="bg-muted-foreground/20" style={`width:${remainingWidth}%`}></div>
				{#if status.hasBaseline}
					<div
						class="absolute inset-y-0 w-0.5 bg-foreground"
						style={`left:${targetPosition}%`}
						data-testid="work-standard-target-marker"
					></div>
				{/if}
			</div>
			<div class="mt-1 flex justify-between text-[9px] text-muted-foreground tabular-nums">
				<span>{formatWorkStatusDuration(0, text)}</span>
				<span>{formatWorkStatusDuration(capacityMinutes, text)}</span>
			</div>
		</div>
		<div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
			<span>
				<i class="mr-1 inline-block size-2 rounded-full bg-foreground"></i>
				{text.workStatus.actual} {formatWorkStatusDuration(status.actualMinutes, text)}
			</span>
			<span>
				<i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>
				{text.workStatus.creditedLeave} {formatWorkStatusDuration(status.creditedLeaveMinutes, text)}
			</span>
			<span>
				<i class="mr-1 inline-block size-2 rounded-full bg-muted-foreground/20"></i>
				{text.workStatus.remaining} {formatWorkStatusDuration(unusedMinutes, text)}
			</span>
		</div>

		<div class="mt-3 space-y-1.5 border-t pt-3 text-[10px]">
			<div class="flex items-center justify-between gap-3">
				<span class="text-muted-foreground">{text.workStatus.nightDuration}</span>
				<span class="font-medium tabular-nums">{formatWorkStatusDuration(status.nightMinutes, text)}</span>
			</div>
			<div class="flex items-center justify-between gap-3">
				<span class="text-muted-foreground">{text.workStatus.overtimeDuration}</span>
				<span class="font-medium tabular-nums">
					{status.hasBaseline ? formatWorkStatusDuration(status.overtimeMinutes, text) : text.workStatus.noBaseline}
				</span>
			</div>
			<div class="flex items-center justify-between gap-3">
				<span class="text-muted-foreground">{text.workStatus.shortfallDuration}</span>
				<span class="font-medium tabular-nums">
					{status.hasBaseline ? formatWorkStatusDuration(status.remainingMinutes, text) : text.workStatus.noBaseline}
				</span>
			</div>
		</div>
	{/if}
</div>
