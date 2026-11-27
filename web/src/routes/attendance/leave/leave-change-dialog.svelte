<script lang="ts">
	import { untrack } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import type { EmployeeLeaveRequest } from './employee-leave-types';
	import LeaveRequestDialogContent from './leave-request-dialog-content.svelte';
	import { LeaveRequestDraft } from './leave-request-draft.svelte';

	type Props = {
		original: EmployeeLeaveRequest | null;
		onClosed: () => void;
	};

	let { original, onClosed }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const employeeLeave = getEmployeeLeaveState();
	const draft = new LeaveRequestDraft();
	const today = $derived(todayDateInTimeZone(myAttendanceToday.summary?.timeZone ?? attendance.summary?.timeZone));

	$effect(() => {
		const opened = original;
		if (!opened) return;
		untrack(() => {
			employeeLeave.mutationErrorMessage = '';
			draft.startFrom(opened, employeeLeave.payload?.leaveTypes ?? [], today);
		});
	});

	function handleOpenChange(open: boolean): void {
		if (!open) onClosed();
	}
</script>

<Dialog.Root open={original !== null} onOpenChange={handleOpenChange}>
	<LeaveRequestDialogContent
		title={text.leave.changeDialogTitle}
		description={text.leave.changeDialogDescription}
		closeLabel={text.close}
		{draft}
		onClose={onClosed}
		testID="leave-change-dialog"
	/>
</Dialog.Root>
