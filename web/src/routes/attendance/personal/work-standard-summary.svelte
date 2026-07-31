<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import {
		calculatePeriodCapacityMinutes,
		formatWorkStatusDuration
	} from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';
	import WorkStandardCapacityBar from './work-standard-capacity-bar.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const defaultCapacityMinutes = 24 * 60;
	const capacityMinutes = $derived(
		status
			? calculatePeriodCapacityMinutes(status.periodStart, status.periodEnd)
			: defaultCapacityMinutes
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

		<WorkStandardCapacityBar
			actualMinutes={status.actualMinutes}
			creditedLeaveMinutes={status.creditedLeaveMinutes}
			{capacityMinutes}
			hasBaseline={status.hasBaseline}
			targetMinutes={status.targetMinutes}
		/>

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
