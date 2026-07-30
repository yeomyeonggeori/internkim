<script lang="ts">
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { EmployeeLeaveRequest } from '../leave/employee-leave-types';
	import { attendanceText } from '../text';
	import { getLeaveManagementState } from './leave-management-state.svelte';

	type Props = {
		request: EmployeeLeaveRequest;
	};

	let { request }: Props = $props();

	const text = createPageText(attendanceText);
	const management = getLeaveManagementState();
	let isOpen = $state(false);
	let startTime = $state('');
	let endTime = $state('');
	let reason = $state('');

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (!open) return;
		startTime = request.startTime ?? '';
		endTime = request.endTime ?? '';
		reason = '';
	}

	async function submit(): Promise<void> {
		if (!startTime || !endTime || !reason.trim()) return;
		await management.correctTime(request.id, {
			startTime,
			endTime,
			reason: reason.trim()
		});
		isOpen = false;
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	<Dialog.Trigger
		class={buttonVariants({ variant: 'outline', size: 'sm' })}
		disabled={management.isMutating}
	>
		{text.management.correctTimeAction}
	</Dialog.Trigger>
	<Dialog.Content class="sm:max-w-md" data-testid="leave-time-correction-dialog">
		<Dialog.Header>
			<Dialog.Title>{text.management.correctTimeTitle}</Dialog.Title>
			<Dialog.Description>{text.management.correctTimeDescription}</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-4 py-2">
			<div class="grid grid-cols-2 gap-3">
				<label class="grid gap-1.5 text-sm font-medium">
					{text.management.startTime}
					<Input type="time" bind:value={startTime} />
				</label>
				<label class="grid gap-1.5 text-sm font-medium">
					{text.management.endTime}
					<Input type="time" bind:value={endTime} />
				</label>
			</div>
			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.reason}
				<Input bind:value={reason} placeholder={text.management.correctTimeReasonPlaceholder} />
			</label>
		</div>
		<Dialog.Footer>
			<Dialog.Close>
				<Button variant="outline">{text.cancel}</Button>
			</Dialog.Close>
			<Button
				onclick={submit}
				disabled={management.isMutating || !startTime || !endTime || !reason.trim()}
			>
				{text.management.saveTimeCorrection}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
