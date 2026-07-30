<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import PaperclipIcon from '@lucide/svelte/icons/paperclip';
	import type { AttendanceText } from '../text';
	import { milliDaysValue } from '../leave/leave-history-model';
	import LeaveApprovalDecisionDialog from './leave-approval-decision-dialog.svelte';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';
	import type {
		LeaveApprovalChange,
		LeaveApprovalRequest,
		LeaveApprovalStatus
	} from './leave-approval-types';

	type Props = {
		request: LeaveApprovalRequest;
		text: AttendanceText['approval'];
		change?: LeaveApprovalChange;
	};

	let { request, text, change }: Props = $props();

	const approval = getLeaveApprovalState();

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

	function statusLabel(status: LeaveApprovalStatus): string {
		switch (status) {
			case 'needsChanges':
				return text.statusNeedsChanges;
			case 'approved':
				return text.statusApproved;
			case 'rejected':
				return text.statusRejected;
			case 'cancelled':
				return text.statusCancelled;
			default:
				return text.statusPending;
		}
	}

	function changeLabel(): string {
		if (!change) return '';
		if (change.change === 'earlyReturn') return text.statusEarlyReturn;
		return statusLabel(change.change);
	}

	function formattedReturnedAt(): string {
		if (!change?.returnedAt) return '';
		return new Intl.DateTimeFormat(text.dateTimeLocale, {
			dateStyle: 'medium',
			timeStyle: 'short'
		}).format(new Date(change.returnedAt));
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

<Card.Root data-testid={`leave-approval-request-${request.id}`}>
	<Card.Header class="gap-3">
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div class="flex min-w-0 items-center gap-3">
				<PersonAvatar
					name={request.employeeEmail}
					email={request.employeeEmail}
					seed={request.employeeEmail}
					class="size-9 shrink-0"
				/>
				<div class="min-w-0">
					<Card.Title class="truncate text-base">{request.employeeEmail}</Card.Title>
					<Card.Description class="mt-1 flex flex-wrap items-center gap-2">
						<span>{leaveTypeName()}</span>
						<span aria-hidden="true">·</span>
						<span>{unitLabel()}</span>
					</Card.Description>
				</div>
			</div>
			<Badge variant={change?.change === 'rejected' ? 'destructive' : 'secondary'}>
				{change ? changeLabel() : statusLabel(request.status)}
			</Badge>
		</div>
	</Card.Header>
	<Card.Content class="space-y-4">
		<div class="grid gap-3 text-sm sm:grid-cols-2">
			<div>
				<p class="text-xs text-muted-foreground">{text.date}</p>
				<p class="mt-1 font-medium tabular-nums">{dateRange()}</p>
				{#if timeRange()}
					<p class="mt-0.5 text-xs tabular-nums text-muted-foreground">{timeRange()}</p>
				{/if}
			</div>
			<div>
				<p class="text-xs text-muted-foreground">{text.deduction}</p>
				<p class="mt-1 font-medium tabular-nums">{days(request.deductionMilliDays)}</p>
			</div>
		</div>

		<div class="space-y-1">
			<p class="text-xs text-muted-foreground">{text.reason}</p>
			<p class="whitespace-pre-wrap text-sm">{request.reason}</p>
		</div>

		<div class="space-y-2">
			<p class="text-xs text-muted-foreground">{text.attachments}</p>
			{#if request.attachments.length === 0}
				<p class="text-sm text-muted-foreground">{text.noAttachments}</p>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each request.attachments as attachment (attachment.id)}
						<a
							href={attachment.downloadURL}
							class="inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-sm hover:bg-muted"
						>
							<PaperclipIcon class="size-3.5" />
							{attachment.fileName}
						</a>
					{/each}
				</div>
			{/if}
		</div>

		{#if change}
			<div class="rounded-md bg-muted/60 p-3 text-sm">
				<p class="font-medium">{changeLabel()}</p>
				<p class="mt-1 text-xs text-muted-foreground">
					{new Intl.DateTimeFormat(text.dateTimeLocale, {
						dateStyle: 'medium',
						timeStyle: 'short'
					}).format(new Date(change.changedAt))}
				</p>
				{#if change.response}
					<p class="mt-2 whitespace-pre-wrap">{change.response}</p>
				{/if}
				{#if change.returnedAt}
					<p class="mt-2">
						<span class="text-muted-foreground">{text.earlyReturnTime}</span>
						<span class="ml-1 font-medium">{formattedReturnedAt()}</span>
					</p>
				{/if}
			</div>
		{/if}
	</Card.Content>
	{#if !change}
		<Card.Footer class="flex flex-wrap justify-end gap-2 border-t pt-4">
			<Button size="sm" onclick={approve} disabled={approval.isMutating}>
				{approval.isMutating ? text.processing : text.approveAction}
			</Button>
			<LeaveApprovalDecisionDialog
				requestID={request.id}
				action="needsChanges"
				{text}
			/>
			<LeaveApprovalDecisionDialog requestID={request.id} action="reject" {text} />
		</Card.Footer>
	{/if}
</Card.Root>
