<script lang="ts">
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
		class={buttonVariants({ variant: 'destructive', size: 'sm' })}
		disabled={approval.isMutating}
	>
		{text.rejectAction}
	</AlertDialog.Trigger>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.rejectConfirmTitle}</AlertDialog.Title>
			<AlertDialog.Description>{text.rejectConfirmDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>{text.close}</AlertDialog.Cancel>
			<AlertDialog.Action onclick={() => void reject()}>
				{text.rejectConfirmAction}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
