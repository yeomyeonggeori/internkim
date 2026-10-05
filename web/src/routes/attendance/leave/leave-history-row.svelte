<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { buttonVariants } from '$lib/components/ui/button';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveRequest, EmployeeLeaveStatus, EmployeeLeaveUnit } from './employee-leave-types';
	import { milliDaysValue, type LeaveHistoryItem } from './leave-history-model';

	type Props = {
		item: LeaveHistoryItem;
		text: AttendanceText['leave'];
		isMutating: boolean;
		onCancel: (request: EmployeeLeaveRequest) => void;
	};

	let { item, text, isMutating, onCancel }: Props = $props();

	function statusLabel(status: EmployeeLeaveStatus): string {
		if (status === 'approved') return text.statusApproved;
		if (status === 'rejected') return text.statusRejected;
		if (status === 'cancelled') return text.statusCancelled;
		return text.statusPending;
	}

	function unitLabel(unit: EmployeeLeaveUnit): string {
		if (unit === 'halfDay') return text.unitHalfDay;
		if (unit === 'quarterDay') return text.unitQuarterDay;
		return text.unitFullDay;
	}

	function requestPeriod(request: EmployeeLeaveRequest): string {
		const dateRange =
			request.endDate && request.endDate !== request.startDate
				? `${request.startDate} – ${request.endDate}`
				: request.startDate;
		if (!request.startTime || !request.endTime) return dateRange;
		return `${dateRange} · ${request.startTime}–${request.endTime}`;
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}
</script>

<article class="flex min-w-0 items-start justify-between gap-3 py-4" data-testid="leave-history-row">
 <div class="min-w-0 space-y-1.5"><p class="text-sm font-medium tabular-nums">{requestPeriod(item.request)}</p><div class="flex flex-wrap items-center gap-2 text-xs"><span>{leaveTypeName(item.request.leaveTypeID,item.request.leaveTypeName)}</span><span class="text-muted-foreground">{unitLabel(item.request.unit)}</span><span class="tabular-nums">{milliDaysValue(item.request.deductionMilliDays)}{text.dayUnit}</span><Badge variant="outline">{statusLabel(item.request.status)}</Badge></div>{#if item.request.reason}<p class="break-words whitespace-pre-line text-xs text-muted-foreground">{item.request.reason}</p>{/if}</div>
	{#if item.request.canCancel}
		<div class="flex shrink-0 items-start">
			<AlertDialog.Root>
				<AlertDialog.Trigger
					class={buttonVariants({ variant: 'ghost', size: 'sm' })}
					disabled={isMutating}
				>
					{text.cancelRequest}
				</AlertDialog.Trigger>
				<AlertDialog.Content>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.cancelConfirmTitle}</AlertDialog.Title>
						<AlertDialog.Description>{text.cancelConfirmDescription}</AlertDialog.Description>
					</AlertDialog.Header>
 <p class="text-sm">{requestPeriod(item.request)} · {leaveTypeName(item.request.leaveTypeID,item.request.leaveTypeName)}</p>
					<AlertDialog.Footer>
						<AlertDialog.Cancel>{text.cancelConfirmClose}</AlertDialog.Cancel>
						<AlertDialog.Action onclick={() => onCancel(item.request)}>
							{text.cancelConfirmAction}
						</AlertDialog.Action>
					</AlertDialog.Footer>
				</AlertDialog.Content>
			</AlertDialog.Root>
		</div>
	{/if}
</article>
