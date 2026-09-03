<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { buttonVariants } from '$lib/components/ui/button';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveRequest, EmployeeLeaveStatus, EmployeeLeaveUnit } from './employee-leave-types';
	import type { LeaveHistoryItem } from './leave-history-model';

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

	function statusVariant(status: EmployeeLeaveStatus): 'default' | 'secondary' | 'destructive' | 'outline' {
		if (status === 'approved') return 'default';
		if (status === 'rejected') return 'destructive';
		if (status === 'cancelled') return 'outline';
		return 'secondary';
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

	function timestamp(value: string): string {
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return value;
		return new Intl.DateTimeFormat(text.dateTimeLocale, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(parsed);
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}
</script>

<article class="grid gap-3 py-4 first:pt-0 last:pb-0" data-testid="leave-history-row">
	<div class="flex min-w-0 items-start justify-between gap-3">
		<div class="min-w-0">
			<div class="flex min-w-0 flex-wrap items-center gap-2">
				<p class="truncate text-sm font-semibold">
					{leaveTypeName(item.request.leaveTypeID, item.request.leaveTypeName)}
				</p>
				<Badge variant={statusVariant(item.request.status)}>
					{statusLabel(item.request.status)}
				</Badge>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">
				{unitLabel(item.request.unit)} · {requestPeriod(item.request)}
			</p>
		</div>
		<p class="shrink-0 text-xs text-muted-foreground">{timestamp(item.occurredAt)}</p>
	</div>

	<div class="grid gap-2 rounded-lg bg-muted/35 px-3 py-3 text-xs">
		<div class="flex gap-3">
			<span class="w-20 shrink-0 text-muted-foreground">{text.reasonLabel}</span>
			<span class="min-w-0 whitespace-pre-wrap">{item.request.reason}</span>
		</div>
	</div>

	{#if item.request.canCancel}
		<div class="flex justify-end">
			<AlertDialog.Root>
				<AlertDialog.Trigger
					class={buttonVariants({ variant: 'outline', size: 'sm' })}
					disabled={isMutating}
				>
					{text.cancelRequest}
				</AlertDialog.Trigger>
				<AlertDialog.Content>
					<AlertDialog.Header>
						<AlertDialog.Title>{text.cancelConfirmTitle}</AlertDialog.Title>
						<AlertDialog.Description>{text.cancelConfirmDescription}</AlertDialog.Description>
					</AlertDialog.Header>
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
