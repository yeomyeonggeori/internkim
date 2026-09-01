<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { getAttendanceApprovalState } from '../approval/attendance-approval-state.svelte';
	import {
		attendanceApprovalDetailLines,
		attendanceApprovalKindLabel
	} from '../approval/attendance-approval-summary';
	import { attendanceText } from '../text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const approval = getAttendanceApprovalState();

	const labels = $derived({
		attendanceAdd: text.approval.attendanceAdd,
		attendanceEdit: text.approval.attendanceEdit,
		attendanceRemove: text.approval.attendanceRemove,
		clockIn: text.records.clockInLabel,
		clockOut: text.records.clockOutLabel
	});
	const myRequests = $derived(approval.requestsFrom(attendance.summary?.currentMemberID));

	function detailLines(detail: Parameters<typeof attendanceApprovalKindLabel>[0]): string[] {
		return attendanceApprovalDetailLines(detail, labels, (eventID) =>
			attendance.summary?.events.find((event) => event.id === eventID)
		);
	}
</script>

{#if myRequests.length > 0}
	<Card.Root data-testid="my-approval-requests">
		<Card.Header>
			<Card.Title class="text-sm">{text.records.myRequestsTitle}</Card.Title>
		</Card.Header>
		<Card.Content class="grid gap-3">
			{#if approval.mutationErrorMessage}
				<p class="text-xs text-destructive">{approval.mutationErrorMessage}</p>
			{/if}
			{#each myRequests as request (request.id)}
				<div class="grid gap-1.5 rounded-lg border border-border/70 px-3 py-2">
					<div class="flex flex-wrap items-center gap-2">
						<Badge variant="secondary">{attendanceApprovalKindLabel(request.detail, labels)}</Badge>
						<span class="text-xs text-muted-foreground">
							{new Date(request.createdAt).toLocaleString(text.approval.dateTimeLocale)}
						</span>
					</div>
					{#each detailLines(request.detail) as line (line)}
						<p class="text-sm font-medium">{line}</p>
					{/each}
					<p class="text-xs text-muted-foreground">{request.reason}</p>
					<div class="flex justify-end">
						<Button
							variant="outline"
							size="xs"
							disabled={approval.isMutating}
							onclick={() => approval.withdraw(request.id)}
							data-testid="my-approval-request-withdraw"
						>
							{text.records.withdrawAction}
						</Button>
					</div>
				</div>
			{/each}
		</Card.Content>
	</Card.Root>
{/if}
