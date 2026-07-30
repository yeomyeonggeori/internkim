<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveRequest, EmployeeLeaveStatus, EmployeeLeaveUnit } from './employee-leave-types';
	import type { LeaveHistoryItem } from './leave-history-model';
	import { milliDaysValue } from './leave-history-model';

	type Props = {
		item: LeaveHistoryItem;
		text: AttendanceText['leave'];
		isMutating: boolean;
		onCancel: (request: EmployeeLeaveRequest) => void;
		onEdit: (request: EmployeeLeaveRequest) => void;
		onResubmit: (request: EmployeeLeaveRequest) => void;
	};

	let { item, text, isMutating, onCancel, onEdit, onResubmit }: Props = $props();

	function statusLabel(status: EmployeeLeaveStatus): string {
		if (status === 'needsChanges') return text.statusNeedsChanges;
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

	function operationLabel(operationType: string): string {
		if (operationType === 'grant') return text.operationGrant;
		if (operationType === 'expire') return text.operationExpire;
		if (operationType === 'adjustment' || operationType === 'adminAdjustment') {
			return text.operationAdjustment;
		}
		if (operationType === 'reserve') return text.operationReserve;
		if (operationType === 'release') return text.operationRelease;
		if (operationType === 'restore') return text.operationRestore;
		if (operationType === 'use') return text.operationUse;
		return text.operationOther;
	}

	function balanceAfter(): string {
		if (item.isUntracked) return text.untracked;
		if (item.balanceAfterMilliDays === undefined) return text.balanceUnavailable;
		return `${milliDaysValue(item.balanceAfterMilliDays)}${text.dayUnit}`;
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
	{#if item.kind === 'request'}
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
			{#if item.request.adminResponse}
				<div class="flex gap-3">
					<span class="w-20 shrink-0 text-muted-foreground">
						{item.request.status === 'rejected'
							? text.rejectionReason
							: text.changesRequestedReason}
					</span>
					<span class="min-w-0 whitespace-pre-wrap">{item.request.adminResponse}</span>
				</div>
			{/if}
			{#if item.request.attachments.length}
				<div class="flex gap-3">
					<span class="w-20 shrink-0 text-muted-foreground">{text.attachmentsLabel}</span>
					<div class="flex min-w-0 flex-wrap gap-x-3 gap-y-1">
						{#each item.request.attachments as attachment (attachment.id)}
							<a class="text-primary hover:underline" href={attachment.downloadURL}>
								{attachment.fileName}
							</a>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<div class="flex flex-wrap items-center justify-between gap-3">
			<p class="text-xs text-muted-foreground">
				{text.balanceAfter} <span class="font-medium text-foreground">{balanceAfter()}</span>
			</p>
			<div class="flex gap-2">
				{#if item.request.canEdit}
					<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={() => onEdit(item.request)}
						disabled={isMutating}
					>
						{text.editRequest}
					</Button>
				{/if}
				{#if item.request.canResubmit}
					<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={() => onResubmit(item.request)}
						disabled={isMutating}
					>
						{text.resubmitAction}
					</Button>
				{/if}
				{#if item.request.canCancel}
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
				{/if}
			</div>
		</div>
	{:else}
		<div class="flex min-w-0 items-start justify-between gap-3">
			<div class="min-w-0">
				<p class="text-sm font-semibold">
					{leaveTypeName(item.entry.leaveTypeID, item.entry.leaveTypeName)}
				</p>
				<p class="mt-1 text-xs text-muted-foreground">
					{operationLabel(item.entry.operationType)}
					<span class="ml-1 font-medium tabular-nums text-foreground">
						{item.entry.deltaMilliDays > 0 ? '+' : ''}{milliDaysValue(item.entry.deltaMilliDays)}{text.dayUnit}
					</span>
				</p>
			</div>
			<p class="shrink-0 text-xs text-muted-foreground">{timestamp(item.occurredAt)}</p>
		</div>
		<p class="text-xs text-muted-foreground">
			{text.balanceAfter} <span class="font-medium text-foreground">{balanceAfter()}</span>
		</p>
	{/if}
</article>
