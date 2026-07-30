<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import { milliDaysValue } from './leave-history-model';

	const text = createPageText(attendanceText);
	const employeeLeave = getEmployeeLeaveState();
	const summary = $derived(employeeLeave.payload?.summary);
	const isUnlimited = $derived(employeeLeave.payload?.balanceTrackingMode === 'unlimited');
	const totalMilliDays = $derived(
		(summary?.usedMilliDays ?? 0) +
			(summary?.reservedMilliDays ?? 0) +
			(summary?.availableMilliDays ?? 0)
	);
	const usedPercent = $derived(segmentPercent(summary?.usedMilliDays ?? 0, totalMilliDays));
	const reservedPercent = $derived(segmentPercent(summary?.reservedMilliDays ?? 0, totalMilliDays));
	const availablePercent = $derived(segmentPercent(summary?.availableMilliDays ?? 0, totalMilliDays));

	function segmentPercent(value: number, total: number): number {
		if (total <= 0) return 0;
		return Math.max(0, (value / total) * 100);
	}

	function days(value: number | undefined): string {
		return `${milliDaysValue(value ?? 0)}${text.leave.dayUnit}`;
	}
</script>

<Card.Root
	class="gap-2"
	role="region"
	aria-label={text.leave.summaryTitle}
	data-testid="leave-balance-summary"
>
	<Card.Header class="pb-0">
		<Card.Title class="text-sm">{text.leave.summaryTitle}</Card.Title>
	</Card.Header>
	<Card.Content class="space-y-2 px-3 pb-3 pt-1">
		<div class="grid grid-cols-3 gap-2">
			<div>
				<p class="text-[11px] text-muted-foreground">{text.leave.summaryUsed}</p>
				<p class="mt-0.5 text-sm font-semibold tabular-nums">{days(summary?.usedMilliDays)}</p>
			</div>
			<div class="text-center">
				<p class="text-[11px] text-muted-foreground">{text.leave.summaryPending}</p>
				<p class="mt-0.5 text-sm font-semibold tabular-nums">{days(summary?.reservedMilliDays)}</p>
			</div>
			<div class="text-right">
				<p class="text-[11px] text-muted-foreground">{text.leave.summaryAvailable}</p>
				<p class="mt-0.5 text-sm font-semibold tabular-nums">
					{isUnlimited ? text.leave.summaryUnlimited : days(summary?.availableMilliDays)}
				</p>
			</div>
		</div>

		{#if !isUnlimited}
			<div
				class="flex h-2 w-full overflow-hidden rounded-full bg-muted"
				role="img"
				aria-label={text.leave.summaryBarLabel
					.replace('{used}', days(summary?.usedMilliDays))
					.replace('{pending}', days(summary?.reservedMilliDays))
					.replace('{available}', days(summary?.availableMilliDays))}
				data-testid="leave-balance-segmented-bar"
			>
				{#if totalMilliDays > 0}
					<span class="h-full bg-primary" style:width={`${usedPercent}%`}></span>
					<span class="h-full bg-primary/40" style:width={`${reservedPercent}%`}></span>
					<span class="h-full bg-muted-foreground/15" style:width={`${availablePercent}%`}></span>
				{/if}
			</div>
		{/if}

		{#if employeeLeave.errorMessage}
			<p class="text-xs text-destructive">{employeeLeave.errorMessage}</p>
		{/if}
	</Card.Content>
</Card.Root>
