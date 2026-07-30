<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount, untrack } from 'svelte';
	import { AttendanceState, setAttendanceState } from './attendance-context.svelte';
	import AttendanceSidebar from './attendance-sidebar.svelte';
	import {
		AttendanceViewState,
		setAttendanceViewState
	} from './attendance-view-state.svelte';
	import {
		LeaveApprovalState,
		setLeaveApprovalState
	} from './approval/leave-approval-state.svelte';
	import {
		EmployeeLeaveState,
		setEmployeeLeaveState
	} from './leave/employee-leave-state.svelte';
	import {
		LeaveManagementState,
		setLeaveManagementState
	} from './management/leave-management-state.svelte';
	import { attendanceText } from './text';

	let { children } = $props();

	const text = createPageText(attendanceText);
	const attendance = new AttendanceState(text.loadFailed);
	const attendanceView = new AttendanceViewState();

	async function refreshAfterEmployeeLeaveMutation(): Promise<void> {
		const refreshes: Promise<void>[] = [attendance.load()];
		if (attendance.summary?.isAdmin) {
			refreshes.push(leaveApproval.load(), leaveManagement.load());
		}
		await Promise.all(refreshes);
	}

	async function refreshAfterApprovalMutation(): Promise<void> {
		await Promise.all([
			attendance.load(),
			employeeLeave.load(),
			leaveManagement.load()
		]);
	}

	async function refreshAfterManagementMutation(): Promise<void> {
		await Promise.all([
			attendance.load(),
			employeeLeave.load(),
			leaveApproval.load()
		]);
	}

	const employeeLeave = new EmployeeLeaveState(text.leave, refreshAfterEmployeeLeaveMutation);
	const leaveApproval = new LeaveApprovalState(text.approval, refreshAfterApprovalMutation);
	const leaveManagement = new LeaveManagementState(
		text.management,
		refreshAfterManagementMutation
	);
	setAttendanceState(attendance);
	setEmployeeLeaveState(employeeLeave);
	setAttendanceViewState(attendanceView);
	setLeaveApprovalState(leaveApproval);
	setLeaveManagementState(leaveManagement);

	function refreshServerClock(): void {
		void attendance.refreshServerClock();
	}

	function refreshVisibleServerClock(): void {
		if (document.visibilityState !== 'visible') return;
		refreshServerClock();
	}

	onMount(() => {
		void attendance.load();
		void employeeLeave.load();
		const releaseRefresh = pageActions.setRefresh(async () => {
			await Promise.all([attendance.load(), employeeLeave.load()]);
		});
		window.addEventListener('focus', refreshServerClock);
		window.addEventListener('pageshow', refreshServerClock);
		document.addEventListener('visibilitychange', refreshVisibleServerClock);
		return () => {
			window.removeEventListener('focus', refreshServerClock);
			window.removeEventListener('pageshow', refreshServerClock);
			document.removeEventListener('visibilitychange', refreshVisibleServerClock);
			releaseRefresh();
		};
	});

	$effect(() => {
		attendance.selectedMonth;
		attendance.chartMode;
		attendance.persistFilters();
	});

	$effect(() => {
		if (attendance.summary?.isAdmin) {
			untrack(() => void leaveApproval.load());
			untrack(() => void leaveManagement.load());
			return;
		}
		untrack(() => leaveApproval.clear());
		untrack(() => leaveManagement.clear());
	});
</script>

<div class="flex min-h-0 min-w-0 flex-1">
	<AttendanceSidebar />

	<div class="min-w-0 flex-1 overflow-y-auto p-3 md:p-6">
		{@render children()}
	</div>
</div>
