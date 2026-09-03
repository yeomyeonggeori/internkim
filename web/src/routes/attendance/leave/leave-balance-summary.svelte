<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import { milliDaysValue } from './leave-history-model';
	import { leaveBalanceSegments } from './leave-balance-segments';

	const text = createPageText(attendanceText);
	const employeeLeave = getEmployeeLeaveState();
	const summary = $derived(employeeLeave.payload?.summary);
	const isUnlimited = $derived(employeeLeave.payload?.balanceTrackingMode === 'unlimited');
	const segments = $derived(leaveBalanceSegments(summary));
	const hasABarToDraw = $derived(
		segments.usedPercent + segments.reservedPercent + segments.availablePercent > 0
	);

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
		<div class="grid gap-2" class:grid-cols-2={isUnlimited} class:grid-cols-3={!isUnlimited}>
			<div>
				<p class="text-[11px] text-muted-foreground">{text.leave.summaryUsed}</p>
				<p class="mt-0.5 text-sm font-semibold tabular-nums">{days(summary?.usedMilliDays)}</p>
			</div>
			<div class:text-center={!isUnlimited} class:text-right={isUnlimited}>
				<p class="text-[11px] text-muted-foreground">{text.leave.summaryPending}</p>
				<p class="mt-0.5 text-sm font-semibold tabular-nums">{days(summary?.reservedMilliDays)}</p>
			</div>
			{#if !isUnlimited}
				<div class="text-right">
					<p class="text-[11px] text-muted-foreground">{text.leave.summaryAvailable}</p>
					<p class="mt-0.5 text-sm font-semibold tabular-nums">
						{days(summary?.availableMilliDays)}
					</p>
				</div>
			{/if}
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
				{#if hasABarToDraw}
					<span class="h-full bg-primary" style:width={`${segments.usedPercent}%`}></span>
					<span class="h-full bg-primary/40" style:width={`${segments.reservedPercent}%`}></span>
					<span class="h-full bg-muted-foreground/15" style:width={`${segments.availablePercent}%`}></span>
				{/if}
			</div>
		{/if}

		{#if employeeLeave.errorMessage}
			<p class="text-xs text-destructive">{employeeLeave.errorMessage}</p>
		{/if}
	</Card.Content>
</Card.Root>
