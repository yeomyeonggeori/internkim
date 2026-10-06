<script lang="ts">
 import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { AttendanceText } from '../text';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';

	type Props = {
		requestID: string;
		text: AttendanceText['approval'];
	};

	let { requestID, text }: Props = $props();

	const approval = getLeaveApprovalState();

	async function reject(): Promise<void> {
		try {
			await approval.decide(requestID, { action: 'reject' });
		} catch {
			return;
		}
	}
</script>

<AlertDialog.Root>
	<AlertDialog.Trigger
		class={buttonVariants({ variant: 'ghost', size: 'sm' })}
		disabled={approval.isMutating}
	>
		{text.rejectAction}
	</AlertDialog.Trigger>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.rejectConfirmTitle}</AlertDialog.Title>
			<AlertDialog.Description>{text.rejectConfirmDescription}</AlertDialog.Description>
		</AlertDialog.Header>
  {@const request = approval.inbox?.pending.find(item => item.id === requestID)}
  {#if request}<div class="flex min-w-0 items-center gap-3 rounded-md border p-3"><PersonAvatar name={request.employeeName} email={request.employeeEmail} /><div class="min-w-0"><p class="truncate font-medium">{request.employeeName}</p><p class="text-sm text-muted-foreground">{request.leaveTypeName} · {request.startDate}{#if request.endDate && request.endDate !== request.startDate} – {request.endDate}{/if}</p></div></div>{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel>{text.close}</AlertDialog.Cancel>
			<AlertDialog.Action onclick={() => void reject()}>
				{text.rejectConfirmAction}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
