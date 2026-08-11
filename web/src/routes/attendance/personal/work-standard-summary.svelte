<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';
	import DurationText from '../shared/duration-text.svelte';
	import WorkStandardCapacityBar from './work-standard-capacity-bar.svelte';
	import { calculateCalendarCapacitySeconds } from './work-standard-capacity';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const totalMinutes = $derived(
		status
			? Math.max(0, status.actualMinutes) +
				Math.max(0, status.provisionalMinutes)
			: 0
	);
	const actualSeconds = $derived(status?.actualSeconds ?? (status?.actualMinutes ?? 0) * 60);
	const provisionalSeconds = $derived(
		status?.provisionalSeconds ?? (status?.provisionalMinutes ?? 0) * 60
	);
	const calendarCapacitySeconds = $derived(
		status?.calendarCapacitySeconds ??
			calculateCalendarCapacitySeconds(status?.periodStart ?? '', status?.periodEnd ?? '')
	);
	const workingCapacitySeconds = $derived(
		status?.workingCapacitySeconds ?? calendarCapacitySeconds
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
		<div class="mb-1">
			<p class="text-[10px] text-muted-foreground">
				{status.periodStart === status.periodEnd
					? status.periodStart
					: `${status.periodStart}–${status.periodEnd}`}
			</p>
			<div class="mt-1 tabular-nums">
				<p class="whitespace-nowrap" data-testid="work-standard-total-row">
					<span class="text-base font-semibold" data-testid="work-standard-total">
						{formatWorkStatusDuration(totalMinutes, text)}
					</span>
				</p>
			</div>
		</div>

		<WorkStandardCapacityBar
			actualMinutes={status.actualMinutes}
			provisionalMinutes={status.provisionalMinutes}
			targetMinutes={status.targetMinutes}
			{actualSeconds}
			{provisionalSeconds}
			{workingCapacitySeconds}
			{calendarCapacitySeconds}
			hasBaseline={status.hasBaseline}
		/>

		{#if status.hasBaseline}
			<div class="mt-3 grid gap-1 border-t pt-2 text-[11px]">
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.nightDuration}</span>
					<DurationText minutes={status.nightMinutes} showZero size="inherit" tone="default" />
				</div>
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.overtimeDuration}</span>
					<DurationText minutes={status.overtimeMinutes} showZero size="inherit" tone="default" />
				</div>
				<div class="flex items-baseline justify-between gap-2">
					<span class="whitespace-nowrap text-muted-foreground">{text.workStatus.shortfallDuration}</span>
					<DurationText minutes={status.remainingMinutes} showZero size="inherit" tone="default" />
				</div>
			</div>
		{/if}
	{/if}
</div>
