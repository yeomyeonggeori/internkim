<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import { milliDaysValue } from './leave-history-model';

	const text = createPageText(attendanceText);
	const employeeLeave = getEmployeeLeaveState();
	const balanceTypes = $derived(
		(employeeLeave.payload?.leaveTypes ?? []).filter(
			(leaveType) =>
				leaveType.isActive &&
				leaveType.balance !== undefined &&
				(leaveType.balanceMode === 'separate' || leaveType.id === 'annual')
		)
	);

	function days(value: number): string {
		return `${milliDaysValue(value)}${text.leave.dayUnit}`;
	}
</script>

<section class="border-b px-4 py-4 sm:px-6" data-testid="leave-type-balances">
	<div>
		<h3 class="text-sm font-semibold">{text.leave.balanceOverviewTitle}</h3>
		<p class="mt-1 text-xs text-muted-foreground">
			{text.leave.balanceOverviewDescription}
		</p>
	</div>
	{#if balanceTypes.length}
		<div class="mt-3 divide-y">
			{#each balanceTypes as leaveType (leaveType.id)}
				<div class="flex items-center justify-between gap-4 py-2.5">
					<span class="text-sm font-medium">{leaveType.name}</span>
					<span class="text-right text-sm tabular-nums">
						<span class="font-semibold">
							{text.leave.balanceOverviewAvailable}
							{days(leaveType.balance?.availableMilliDays ?? 0)}
						</span>
						{#if (leaveType.balance?.reservedMilliDays ?? 0) > 0}
							<span class="ml-2 text-xs text-muted-foreground">
								{text.leave.balanceOverviewPending}
								{days(leaveType.balance?.reservedMilliDays ?? 0)}
							</span>
						{/if}
					</span>
				</div>
			{/each}
		</div>
	{:else}
		<p class="mt-3 text-sm text-muted-foreground">
			{text.leave.balanceOverviewEmpty}
		</p>
	{/if}
</section>
