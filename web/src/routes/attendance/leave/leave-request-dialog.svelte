<script lang="ts">
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import LeaveRequestDialogContent from './leave-request-dialog-content.svelte';
	import { LeaveRequestDraft as LeaveRequestDraftState } from './leave-request-draft.svelte';

	type Props = {
		showTrigger?: boolean;
		onClosed?: () => void;
	};

	let { showTrigger = true, onClosed = () => undefined }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const employeeLeave = getEmployeeLeaveState();
	const draft = new LeaveRequestDraftState();
	let isOpen = $state(false);
	const today = $derived(todayDateInTimeZone(myAttendanceToday.summary?.timeZone ?? attendance.summary?.timeZone));

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

</script>

<Dialog.Root open={isOpen} onOpenChange={handleOpenChange}>
	{#if showTrigger}
		<Dialog.Trigger class={cn(buttonVariants({ variant: 'default', size: 'sm' }))}>
			{text.leave.registerAction}
		</Dialog.Trigger>
	{/if}
	<LeaveRequestDialogContent
		title={text.leave.dialogTitle}
		description={text.leave.dialogDescription}
		closeLabel={text.close}
		{draft}
		onClose={close}
		testID="leave-request-dialog"
	/>
</Dialog.Root>
