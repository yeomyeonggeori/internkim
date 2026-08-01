<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';
	import DurationText from '../shared/duration-text.svelte';
	import WorkStandardCapacityBar from './work-standard-capacity-bar.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const totalMinutes = $derived(
		status
			? Math.max(0, status.actualMinutes) +
				Math.max(0, status.provisionalMinutes) +
				Math.max(0, status.paidLeaveMinutes)
			: 0
	);
	const displayedActualMinutes = $derived(
		status ? Math.max(0, status.actualMinutes) + Math.max(0, status.provisionalMinutes) : 0
	);
	const uncreditedLeaveMinutes = $derived(
		status ? Math.max(0, status.paidLeaveMinutes - status.creditedLeaveMinutes) : 0
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
					<p
						class="flex items-baseline gap-1 whitespace-nowrap"
						data-testid="work-standard-total-row"
					>
						<span class="text-base font-semibold" data-testid="work-standard-total">
							{formatWorkStatusDuration(totalMinutes, text)}
						</span>
						<span class="text-[9px] text-muted-foreground">
							/ {text.workStatus.target} {formatWorkStatusDuration(status.targetMinutes, text)}
						</span>
					</p>
				</div>
			{:else}
				<p class="mt-1 text-lg font-semibold text-muted-foreground">{text.workStatus.noBaseline}</p>
			{/if}
		</div>

		{#if status.hasBaseline}
			<WorkStandardCapacityBar
				actualMinutes={status.actualMinutes}
				provisionalMinutes={status.provisionalMinutes}
				paidLeaveMinutes={status.paidLeaveMinutes}
				creditedLeaveMinutes={status.creditedLeaveMinutes}
				targetMinutes={status.targetMinutes}
			/>
		{:else}
			<div class="grid grid-cols-2 gap-3 text-[11px]" data-testid="work-standard-no-baseline-values">
				<div>
					<p class="text-muted-foreground">{text.workStatus.actual}</p>
					<p class="mt-1"><DurationText minutes={displayedActualMinutes} showZero size="inherit" tone="default" /></p>
				</div>
				<div>
					<p class="text-muted-foreground">{text.workStatus.leave}</p>
					<p class="mt-1"><DurationText minutes={status.paidLeaveMinutes} showZero size="inherit" tone="default" /></p>
				</div>
			</div>
		{/if}

		<div class="mt-3 grid gap-1 border-t pt-2 text-[11px]">
			<div class="flex items-baseline justify-between gap-2">
				<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.nightDuration}</span>
				<DurationText minutes={status.nightMinutes} showZero size="inherit" tone="default" />
			</div>
			<div class="flex items-baseline justify-between gap-2">
				<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.overtimeDuration}</span>
				{#if status.hasBaseline}
					<DurationText minutes={status.overtimeMinutes} showZero size="inherit" tone="default" />
				{:else}
					<span class="whitespace-nowrap font-medium">{text.workStatus.noBaseline}</span>
				{/if}
			</div>
			<div class="flex items-baseline justify-between gap-2">
				<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.shortfallDuration}</span>
				{#if status.hasBaseline}
					<DurationText minutes={status.remainingMinutes} showZero size="inherit" tone="default" />
				{:else}
					<span class="whitespace-nowrap font-medium">{text.workStatus.noBaseline}</span>
				{/if}
			</div>
			{#if uncreditedLeaveMinutes > 0}
				<div
					class="flex items-baseline justify-between gap-2"
					data-testid="work-standard-uncredited-leave"
				>
					<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.uncreditedLeaveDuration}</span>
					<DurationText minutes={uncreditedLeaveMinutes} size="inherit" tone="default" />
				</div>
			{/if}
		</div>
	{/if}
</div>
