<script lang="ts">
	import DatePicker from '$lib/components/date-picker.svelte';
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
	let startDate = $state('');
	let endDate = $state('');
	let reason = $state('');
	const isReady = $derived(startDate !== '' && endDate >= startDate && reason.trim() !== '');

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (!open) return;
		startDate = request.startDate;
		endDate = request.endDate || request.startDate;
		reason = '';
	}

	function moveStart(next: string): void {
		startDate = next;
		if (endDate < next) endDate = next;
	}

	async function submit(): Promise<void> {
		if (!isReady) return;
		await management.correctDays(request.id, { startDate, endDate, reason });
		isOpen = false;
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	<Dialog.Trigger
		class={buttonVariants({ variant: 'outline', size: 'sm' })}
		disabled={management.isMutating}
	>
		{text.management.correctDaysAction}
	</Dialog.Trigger>
	<Dialog.Content class="sm:max-w-md" data-testid="leave-days-correction-dialog">
		<Dialog.Header>
			<Dialog.Title>{text.management.correctDaysTitle}</Dialog.Title>
			<Dialog.Description>{text.management.correctDaysDescription}</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-4 py-2">
			<div class="grid grid-cols-2 gap-3">
				<label class="grid gap-1.5 text-sm font-medium" data-testid="leave-days-correction-start">
					{text.management.startDate}
					<DatePicker bind:value={() => startDate, moveStart} />
				</label>
				<label class="grid gap-1.5 text-sm font-medium" data-testid="leave-days-correction-end">
					{text.management.endDate}
					<DatePicker bind:value={endDate} min={startDate} />
				</label>
			</div>
			<label class="grid gap-1.5 text-sm font-medium">
				{text.management.reason}
				<Input bind:value={reason} placeholder={text.management.correctDaysReasonPlaceholder} />
			</label>
		</div>
		<Dialog.Footer>
			<Dialog.Close>
				<Button variant="outline">{text.cancel}</Button>
			</Dialog.Close>
			<Button onclick={submit} disabled={management.isMutating || !isReady}>
				{text.management.saveDaysCorrection}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
