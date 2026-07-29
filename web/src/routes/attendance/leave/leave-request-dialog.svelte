<script lang="ts">
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import type { EmployeeLeaveRequest } from './employee-leave-types';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import LeaveRequestForm from './leave-request-form.svelte';
	import { LeaveRequestDraft as LeaveRequestDraftState } from './leave-request-draft.svelte';

	type Props = {
		request?: EmployeeLeaveRequest;
		showTrigger?: boolean;
		onClosed?: () => void;
	};

	let { request, showTrigger = true, onClosed = () => undefined }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const employeeLeave = getEmployeeLeaveState();
	const draft = new LeaveRequestDraftState();
	let isOpen = $state(false);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));

	function resetDialog(): void {
		draft.reset(employeeLeave.payload?.leaveTypes ?? [], today);
	}

	function handleOpenChange(open: boolean): void {
		isOpen = open;
		if (!open) {
			resetDialog();
			onClosed();
			return;
		}
		employeeLeave.mutationErrorMessage = '';
		resetDialog();
		void employeeLeave.load();
	}

	function close(): void {
		isOpen = false;
		resetDialog();
		onClosed();
	}

	$effect(() => {
		if (!request) return;
		draft.loadRequest(request);
		employeeLeave.mutationErrorMessage = '';
		isOpen = true;
	});

	function title(): string {
		if (!request) return text.leave.dialogTitle;
		return request.canEdit ? text.leave.editTitle : text.leave.resubmitTitle;
	}

	function description(): string {
		if (!request) return text.leave.dialogDescription;
		return request.canEdit ? text.leave.editDescription : text.leave.resubmitDescription;
	}
</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	{#if showTrigger}
		<Dialog.Trigger class={cn(buttonVariants({ variant: 'outline' }), 'w-full')}>
			{text.leave.registerAction}
		</Dialog.Trigger>
	{/if}
	<Dialog.Content
		class="top-0 left-0 h-svh max-h-none max-w-none translate-x-0 translate-y-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden rounded-none p-0 sm:top-1/2 sm:left-1/2 sm:h-[min(90vh,56rem)] sm:max-w-3xl sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl"
		closeLabel={text.close}
		data-testid="leave-request-dialog"
	>
		<div class="border-b bg-popover px-4 py-5 sm:px-6">
			<Dialog.Header class="pr-10">
				<Dialog.Title class="text-xl">
					{title()}
				</Dialog.Title>
				<Dialog.Description>
					{description()}
				</Dialog.Description>
			</Dialog.Header>
		</div>
		<div class="min-h-0 overflow-y-auto">
			<LeaveRequestForm {draft} onSubmitted={close} onClose={close} />
		</div>
	</Dialog.Content>
</Dialog.Root>
