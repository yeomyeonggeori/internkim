<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';

	const text = createPageText(attendanceText);
	const workStatus = getWorkStatusState();
	const status = $derived(workStatus.payload?.personal);
	const scaleMinutes = $derived(
		status
			? Math.max(status.hasBaseline ? status.targetMinutes : status.actualMinutes, 1)
			: 1
	);
	const actualWidth = $derived(
		status
			? (Math.min(
					status.actualMinutes,
					status.hasBaseline ? status.targetMinutes : status.actualMinutes
				) /
					scaleMinutes) *
				100
			: 0
	);
	const leaveWidth = $derived(
		status?.hasBaseline ? (status.creditedLeaveMinutes / scaleMinutes) * 100 : 0
	);
	const remainingWidth = $derived(
		status?.hasBaseline ? (status.remainingMinutes / scaleMinutes) * 100 : 0
	);
</script>

{#if status && !workStatus.errorMessage}
	<div class="mt-3 border-t pt-3" data-testid="personal-work-standard">
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
			{#if status.hasBaseline}
				<span><i class="mr-1 inline-block size-2 rounded-full bg-blue-500"></i>{text.workStatus.creditedLeave}</span>
				<span><i class="mr-1 inline-block size-2 rounded-full bg-muted-foreground/20"></i>{text.workStatus.remaining}</span>
			{/if}
		</div>
	</div>
{/if}
