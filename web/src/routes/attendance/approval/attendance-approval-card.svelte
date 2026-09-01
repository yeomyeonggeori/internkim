<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Textarea } from '$lib/components/ui/textarea';
	import { getAttendanceState } from '../attendance-context.svelte';
	import type { AttendanceText } from '../text';
	import { getAttendanceApprovalState } from './attendance-approval-state.svelte';
	import {
		attendanceApprovalDetailLines,
		attendanceApprovalKindLabel
	} from './attendance-approval-summary';
	import type { AttendanceApprovalRequest } from './attendance-approval-types';

	type Props = {
		request: AttendanceApprovalRequest;
		text: AttendanceText;
	};

	let { request, text }: Props = $props();

	const approval = getAttendanceApprovalState();
	const attendance = getAttendanceState();
	let note = $state('');

	const labels = $derived({
		attendanceAdd: text.approval.attendanceAdd,
		attendanceEdit: text.approval.attendanceEdit,
		attendanceRemove: text.approval.attendanceRemove,
		clockIn: text.records.clockInLabel,
		clockOut: text.records.clockOutLabel
	});
	const kindLabel = $derived(attendanceApprovalKindLabel(request.detail, labels));
	const detailLines = $derived(
		attendanceApprovalDetailLines(request.detail, labels, (eventID) =>
			attendance.summary?.events.find((event) => event.id === eventID)
		)
	);

	async function decide(decision: 'approved' | 'rejected'): Promise<void> {
		try {
			await approval.decide(request.id, decision, note);
			note = '';
		} catch {
			return;
		}
	}
</script>

<Card.Root data-testid="attendance-approval-card">
	<Card.Header>
		<Card.Title class="flex flex-wrap items-center gap-2 text-base">
			<Badge variant="secondary">{kindLabel}</Badge>
			<span>{request.askedBy}</span>
		</Card.Title>
		<Card.Description>{new Date(request.createdAt).toLocaleString(text.approval.dateTimeLocale)}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-3">
		<div class="grid gap-1">
			{#each detailLines as line (line)}
				<p class="text-sm font-medium" data-testid="attendance-approval-detail">{line}</p>
			{/each}
		</div>
		<div class="grid gap-1">
			<span class="text-xs font-medium text-muted-foreground">{text.approval.reason}</span>
			<p class="text-sm">{request.reason}</p>
		</div>
		<label class="grid gap-1 text-xs font-medium text-muted-foreground">
			<span>{text.approval.responseLabel} <span class="font-normal">{text.approval.optional}</span></span>
			<Textarea
				bind:value={note}
				placeholder={text.approval.responsePlaceholder}
				disabled={approval.isMutating}
				class="min-h-16 text-sm"
			/>
		</label>
	</Card.Content>
	<Card.Footer class="justify-end gap-2">
		<Button
			variant="destructive"
			size="sm"
			disabled={approval.isMutating}
			onclick={() => decide('rejected')}
			data-testid="attendance-approval-reject"
		>
			{text.approval.rejectAction}
		</Button>
		<Button
			size="sm"
			disabled={approval.isMutating}
			onclick={() => decide('approved')}
			data-testid="attendance-approval-approve"
		>
			{text.approval.approveAction}
		</Button>
	</Card.Footer>
</Card.Root>
