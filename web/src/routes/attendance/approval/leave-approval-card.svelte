<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AttendanceText } from '../text';
	import { milliDaysValue } from '../leave/leave-history-model';
	import LeaveApprovalDecisionDialog from './leave-approval-decision-dialog.svelte';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';
	import type { LeaveApprovalRequest } from './leave-approval-types';

	type Props = {
		request: LeaveApprovalRequest;
		text: AttendanceText['approval'];
	};

	let { request, text }: Props = $props();

	const approval = getLeaveApprovalState();
	const employeeName = $derived(request.employeeName || request.employeeEmail);

	function days(value: number): string {
		return `${milliDaysValue(value)}${text.dayUnit}`;
	}

	function dateRange(): string {
		return request.endDate && request.endDate !== request.startDate
			? `${request.startDate} – ${request.endDate}`
			: request.startDate;
	}

	function timeRange(): string {
		if (!request.startTime || !request.endTime) return '';
		return `${request.startTime} – ${request.endTime}`;
	}

	function unitLabel(): string {
		switch (request.unit) {
			case 'halfDay':
				return text.unitHalfDay;
			case 'quarterDay':
				return text.unitQuarterDay;
			default:
				return text.unitFullDay;
		}
	}

	function leaveTypeName(): string {
		return localizedLeaveTypeName(
			request.leaveTypeID,
			request.leaveTypeName,
			currentLocale.value
		);
	}

	async function approve(): Promise<void> {
		try {
			await approval.decide(request.id, { action: 'approve' });
		} catch {
			return;
		}
	}
</script>

<article class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-x-4 gap-y-3 py-4 sm:grid-cols-[11rem_minmax(0,1fr)_auto]" data-testid={`leave-approval-request-${request.id}`}>
 <div class="col-span-2 flex min-w-0 items-center gap-3 sm:col-span-1"><PersonAvatar name={employeeName} email={request.employeeEmail} class="size-8" /><div class="min-w-0"><p class="truncate text-sm font-medium">{employeeName}</p><p class="truncate text-xs text-muted-foreground">{request.employeeEmail}</p></div></div>
 <div class="min-w-0 space-y-1.5 text-sm">
  <p class="font-medium tabular-nums">{dateRange()}</p>
  {#if timeRange()}<p class="text-xs tabular-nums text-muted-foreground">{timeRange()}</p>{/if}
  <div class="flex flex-wrap items-center gap-2"><Badge variant="outline">{leaveTypeName()}</Badge><span class="text-xs text-muted-foreground">{unitLabel()}</span><span class="tabular-nums">{days(request.deductionMilliDays)}</span></div>
  {#if request.reason}<p class="break-words whitespace-pre-line text-xs text-muted-foreground" aria-label={text.reason}>{request.reason}</p>{/if}
 </div>
 <div class="flex items-center gap-1.5 self-center"><LeaveApprovalDecisionDialog requestID={request.id} {text} /><Button size="sm" onclick={approve} disabled={approval.isMutating}>{approval.isMutating ? text.processing : text.approveAction}</Button></div>
</article>
