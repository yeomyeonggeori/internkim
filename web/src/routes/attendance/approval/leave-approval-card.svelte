<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import type { AttendanceText } from '../text';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { milliDaysValue } from '../leave/leave-history-model';
	import { leaveApprovalEmployeeName } from './leave-approval-employee-name';
	import LeaveApprovalDecisionDialog from './leave-approval-decision-dialog.svelte';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';
	import type { LeaveApprovalRequest, LeaveApprovalStatus } from './leave-approval-types';

	type Props = {
		request: LeaveApprovalRequest;
		text: AttendanceText['approval'];
	};

	let { request, text }: Props = $props();

	const approval = getLeaveApprovalState();
	const attendance = getAttendanceState();
	const employeeName = $derived(
		leaveApprovalEmployeeName(attendance.summary?.members ?? [], request.employeeEmail)
	);

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
					name={employeeName}
					email={request.employeeEmail}
					seed={request.employeeEmail}
					class="size-9 shrink-0"
				/>
				<div class="min-w-0">
					<Card.Title class="truncate text-base">{employeeName}</Card.Title>
					<Card.Description class="mt-1 flex flex-wrap items-center gap-2">
						<span>{leaveTypeName()}</span>
						<span aria-hidden="true">·</span>
						<span>{unitLabel()}</span>
					</Card.Description>
				</div>
			</div>
			<Badge variant="secondary">{statusLabel(request.status)}</Badge>
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

	</Card.Content>
	<Card.Footer class="flex flex-wrap justify-end gap-2 border-t pt-4">
		<LeaveApprovalDecisionDialog requestID={request.id} {text} />
		<Button size="sm" onclick={approve} disabled={approval.isMutating}>
			{approval.isMutating ? text.processing : text.approveAction}
		</Button>
	</Card.Footer>
</Card.Root>
