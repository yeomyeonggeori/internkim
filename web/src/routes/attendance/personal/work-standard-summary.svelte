<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const scaleMinutes = $derived(
		status ? Math.max(status.targetMinutes + status.overtimeMinutes, status.actualMinutes, 1) : 1
	);
	const baselineActualMinutes = $derived(
		status ? Math.min(status.actualMinutes, status.targetMinutes || status.actualMinutes) : 0
	);
	const actualWidth = $derived((baselineActualMinutes / scaleMinutes) * 100);
	const leaveWidth = $derived(((status?.creditedLeaveMinutes ?? 0) / scaleMinutes) * 100);
	const remainingWidth = $derived(((status?.remainingMinutes ?? 0) / scaleMinutes) * 100);
	const overtimeWidth = $derived(((status?.overtimeMinutes ?? 0) / scaleMinutes) * 100);

	function statusMessage(value: string): string {
		const messages: Record<string, string> = {
			working: text.workStatus.statusWorking,
			needsReview: text.workStatus.statusNeedsReview,
			coreTimeMissed: text.workStatus.statusCoreTimeMissed,
			late: text.workStatus.statusLate,
			earlyLeave: text.workStatus.statusEarlyLeave,
			lateAndEarlyLeave: text.workStatus.statusLateAndEarlyLeave
		};
		return messages[value] ?? '';
	}
</script>

<div class="mt-3 border-t pt-3" data-testid="personal-work-standard">
	<div class="mb-2 flex items-center justify-between gap-2">
		<p class="text-[11px] font-medium">{text.workStatus.standard}</p>
		{#if workStatus.isLoading}
			<span class="text-[10px] text-muted-foreground">{text.loading}</span>
		{:else if status}
			<span class="text-[10px] text-muted-foreground">
				{status.periodStart === status.periodEnd
					? status.periodStart
					: `${status.periodStart}–${status.periodEnd}`}
			</span>
		{/if}
	</div>

	{#if workStatus.errorMessage}
		<p class="text-[10px] text-destructive">{text.workStatus.loadFailed}</p>
	{:else if status}
		{#if status.hasBaseline}
			<div
				class="flex h-2.5 overflow-hidden rounded-full bg-muted"
				aria-label={`${text.workStatus.actual} ${formatWorkStatusDuration(status.actualMinutes, text)}, ${text.workStatus.creditedLeave} ${formatWorkStatusDuration(status.creditedLeaveMinutes, text)}, ${text.workStatus.remaining} ${formatWorkStatusDuration(status.remainingMinutes, text)}`}
			>
				<div class="bg-foreground" style={`width:${actualWidth}%`}></div>
				<div class="bg-blue-500" style={`width:${leaveWidth}%`}></div>
				<div class="bg-muted-foreground/20" style={`width:${remainingWidth}%`}></div>
				<div class="bg-orange-500" style={`width:${overtimeWidth}%`}></div>
			</div>
		{:else}
			<div class="h-2.5 overflow-hidden rounded-full bg-muted">
				{#if status.actualMinutes > 0}
					<div class="h-full w-full bg-foreground"></div>
				{/if}
			</div>
			<p class="mt-2 text-[10px] text-muted-foreground">
				{status.actualMinutes > 0 ? text.workStatus.autonomousDescription : text.noData}
			</p>
		{/if}

		{#if statusMessage(status.status)}
			<p
				class="mt-2 rounded-md bg-muted px-2 py-1.5 text-[10px]"
				class:text-destructive={status.needsReview}
				role="status"
			>
				{statusMessage(status.status)}
				{#if status.isWorking && status.provisionalMinutes > 0}
					· {text.workStatus.provisional} {formatWorkStatusDuration(status.provisionalMinutes, text)}
				{/if}
			</p>
		{/if}

		<div class="mt-2 grid gap-1 text-[10px]">
			<div class="flex justify-between gap-2">
				<span class="text-muted-foreground">{text.workStatus.actual}</span>
				<span>{formatWorkStatusDuration(status.actualMinutes, text)}</span>
			</div>
			{#if status.hasBaseline}
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.target}</span>
					<span>{formatWorkStatusDuration(status.targetMinutes, text)}</span>
				</div>
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.paidLeave}</span>
					<span>{formatWorkStatusDuration(status.paidLeaveMinutes, text)}</span>
				</div>
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.creditedLeave}</span>
					<span>{formatWorkStatusDuration(status.creditedLeaveMinutes, text)}</span>
				</div>
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.fulfilled}</span>
					<span>{formatWorkStatusDuration(status.fulfilledMinutes, text)}</span>
				</div>
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.remaining}</span>
					<span>{formatWorkStatusDuration(status.remainingMinutes, text)}</span>
				</div>
				<div class="flex justify-between gap-2">
					<span class="text-muted-foreground">{text.workStatus.overtime}</span>
					<span>{formatWorkStatusDuration(status.overtimeMinutes, text)}</span>
				</div>
			{/if}
			<div class="flex justify-between gap-2">
				<span class="text-muted-foreground">{text.workStatus.night}</span>
				<span>{formatWorkStatusDuration(status.nightMinutes, text)}</span>
			</div>
		</div>
	{/if}
</div>
