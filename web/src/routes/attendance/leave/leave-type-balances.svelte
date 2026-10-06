<script lang="ts">
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import { milliDaysValue } from './leave-history-model';
	import { Skeleton } from '$lib/components/ui/skeleton';

	const text = createPageText(attendanceText);
	const employeeLeave = getEmployeeLeaveState();
	const isUnlimited = $derived(employeeLeave.payload?.balanceTrackingMode === 'unlimited');
	const balanceTypes = $derived(
		(employeeLeave.payload?.leaveTypes ?? []).filter(
			(leaveType) =>
				leaveType.isActive &&
				leaveType.balance !== undefined &&
				(isUnlimited || leaveType.balanceMode === 'separate' || leaveType.id === 'annual')
		)
	);

	function days(value: number): string {
		return `${milliDaysValue(value)}${text.leave.dayUnit}`;
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}
</script>


<section data-testid="leave-type-balances" aria-label={text.leave.balanceOverviewTitle}>
 {#if !employeeLeave.payload && !employeeLeave.errorMessage}<div role="status" aria-busy="true" aria-label={text.loading} class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{#each [0, 1, 2] as card (card)}<div aria-hidden="true" class="space-y-3 rounded-lg border px-4 py-3"><Skeleton class="h-5 w-20" /><div class="flex items-baseline gap-2"><Skeleton class="h-3 w-12" /><Skeleton class="h-8 w-20" /></div></div>{/each}</div>
 {:else if balanceTypes.length}<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{#each balanceTypes as leaveType (leaveType.id)}
  <div class="rounded-lg border px-4 py-3"><p class="text-sm font-medium">{leaveTypeName(leaveType.id,leaveType.name)}</p><div class="mt-2 flex flex-wrap items-baseline justify-between gap-2"><p class="tabular-nums"><span class="mr-2 text-xs text-muted-foreground">{isUnlimited ? text.leave.balanceOverviewUsed : text.leave.balanceOverviewAvailable}</span><span class="text-2xl font-semibold">{days(isUnlimited ? leaveType.balance?.usedMilliDays ?? 0 : leaveType.balance?.availableMilliDays ?? 0)}</span></p>{#if (leaveType.balance?.reservedMilliDays ?? 0) > 0}<p class="text-xs text-muted-foreground">{text.leave.balanceOverviewPending} {days(leaveType.balance?.reservedMilliDays ?? 0)}</p>{/if}</div></div>
 {/each}</div>{:else}<p class="text-sm text-muted-foreground">{text.leave.balanceOverviewEmpty}</p>{/if}
</section>
