<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Textarea } from '$lib/components/ui/textarea';
	import { cn } from '$lib/utils';
	import type { AttendanceText } from '../text';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';
	import type { LeaveApprovalAction } from './leave-approval-types';

	type Props = {
		requestID: string;
		action: Extract<LeaveApprovalAction, 'needsChanges' | 'reject'>;
		text: AttendanceText['approval'];
	};

	let { requestID, action, text }: Props = $props();

	const approval = getLeaveApprovalState();
	let isOpen = $state(false);
	let isConfirmationOpen = $state(false);
	let response = $state('');
	const isRequired = $derived(action === 'needsChanges');
	const title = $derived(
		action === 'needsChanges' ? text.needsChangesTitle : text.rejectTitle
	);
	const description = $derived(
		action === 'needsChanges' ? text.needsChangesDescription : text.rejectDescription
	);
	const triggerLabel = $derived(
		action === 'needsChanges' ? text.needsChangesAction : text.rejectAction
	);
	const submitLabel = $derived(
		action === 'needsChanges' ? text.needsChangesSubmit : text.rejectSubmit
	);

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (!open) response = '';
	}

	async function submitDecision(): Promise<void> {
		if (isRequired && !response.trim()) return;
		try {
			await approval.decide(requestID, { action, response });
			isOpen = false;
			response = '';
		} catch {
			return;
		}
	}

	function submit(): void {
		if (action === 'reject') {
			isConfirmationOpen = true;
			return;
		}
		void submitDecision();
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	<Dialog.Trigger
		class={cn(
			buttonVariants({ variant: action === 'reject' ? 'destructive' : 'outline', size: 'sm' })
		)}
		disabled={approval.isMutating}
	>
		{triggerLabel}
	</Dialog.Trigger>
	<Dialog.Content class="sm:max-w-lg" closeLabel={text.close}>
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>{description}</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-2 py-2">
			<label for={`leave-approval-response-${requestID}-${action}`} class="text-sm font-medium">
				{text.responseLabel}{#if !isRequired}
					<span class="ml-1 font-normal text-muted-foreground">{text.optional}</span>
				{/if}
			</label>
			<Textarea
				id={`leave-approval-response-${requestID}-${action}`}
				bind:value={response}
				placeholder={text.responsePlaceholder}
				class="min-h-28 resize-y"
			/>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => handleOpenChange(false)} disabled={approval.isMutating}>
				{text.close}
			</Button>
			<Button
				variant={action === 'reject' ? 'destructive' : 'default'}
				onclick={submit}
				disabled={approval.isMutating || (isRequired && !response.trim())}
			>
				{approval.isMutating ? text.processing : submitLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

{#if action === 'reject'}
	<AlertDialog.Root bind:open={isConfirmationOpen}>
		<AlertDialog.Content>
			<AlertDialog.Header>
				<AlertDialog.Title>{text.rejectConfirmTitle}</AlertDialog.Title>
				<AlertDialog.Description>{text.rejectConfirmDescription}</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>{text.close}</AlertDialog.Cancel>
				<AlertDialog.Action onclick={() => void submitDecision()}>
					{text.rejectConfirmAction}
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/if}
