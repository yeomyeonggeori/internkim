<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const scaleMinutes = $derived(status ? Math.max(status.targetMinutes, 1) : 1);
	const actualWidth = $derived(
		status ? (Math.min(status.actualMinutes, status.targetMinutes) / scaleMinutes) * 100 : 0
	);
	const leaveWidth = $derived(
		status ? (status.creditedLeaveMinutes / scaleMinutes) * 100 : 0
	);
	const remainingWidth = $derived(
		status ? (status.remainingMinutes / scaleMinutes) * 100 : 0
	);

	function statusMessage(value: string): string {
		const messages: Record<string, string> = {
			working: text.workStatus.statusWorking,
			needsReview: text.workStatus.statusNeedsReview,
			coreTimeMissed: text.workStatus.statusCoreTimeMissed,
			late: text.workStatus.statusLate,
			earlyLeave: text.workStatus.statusEarlyLeave,
			lateAndEarlyLeave: text.workStatus.statusLateAndEarlyLeave,
			remaining: text.workStatus.statusRemaining,
			fulfilled: text.workStatus.statusFulfilled,
			overtime: text.workStatus.statusOvertime,
			actualOnly: text.workStatus.statusActualOnly
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

		{#if status.hasBaseline}
			<div
				class="flex h-2.5 overflow-hidden rounded-full bg-muted"
				aria-label={`${text.workStatus.actual} ${formatWorkStatusDuration(status.actualMinutes, text)}, ${text.workStatus.creditedLeave} ${formatWorkStatusDuration(status.creditedLeaveMinutes, text)}, ${text.workStatus.remaining} ${formatWorkStatusDuration(status.remainingMinutes, text)}`}
			>
				<div class="bg-foreground" style={`width:${actualWidth}%`}></div>
				<div class="bg-blue-500" style={`width:${leaveWidth}%`}></div>
				<div class="bg-muted-foreground/20" style={`width:${remainingWidth}%`}></div>
			</div>
			<div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-muted-foreground">
				<span><i class="mr-1 inline-block size-2 rounded-full bg-foreground"></i>{text.workStatus.actual}</span>
				<span><i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>{text.workStatus.creditedLeave}</span>
				<span><i class="mr-1 inline-block size-2 rounded-full bg-muted-foreground/20"></i>{text.workStatus.remaining}</span>
			</div>
		{:else}
			<div class="h-2.5 overflow-hidden rounded-full bg-muted">
				{#if status.actualMinutes > 0}<div class="h-full w-full bg-foreground"></div>{/if}
			</div>
			<p class="mt-1.5 text-[10px] text-muted-foreground">{text.workStatus.actual}</p>
		{/if}

		<div class="mt-3 grid grid-cols-2 gap-2 text-[10px]">
			<div class="rounded-md border px-2 py-1.5">
				<p class="text-muted-foreground">{text.workStatus.night}</p>
				<p class="mt-0.5 font-medium tabular-nums">{formatWorkStatusDuration(status.nightMinutes, text)}</p>
			</div>
			<div class="rounded-md border px-2 py-1.5">
				<p class="text-muted-foreground">{text.workStatus.overtime}</p>
				<p class="mt-0.5 font-medium tabular-nums">
					{status.hasBaseline ? formatWorkStatusDuration(status.overtimeMinutes, text) : text.workStatus.noBaseline}
				</p>
			</div>
		</div>

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
	{/if}
</div>
