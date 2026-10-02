<script lang="ts">
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount, untrack } from 'svelte';
	import { AttendanceState, setAttendanceState } from './attendance-context.svelte';
	import AttendanceSidebar from './attendance-sidebar.svelte';
	import {
		AttendanceViewState,
		type AttendanceWorkspaceView,
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
		HandWrittenState,
		setHandWrittenState
	} from './hand-written/hand-written-state.svelte';
	import {
		LeaveManagementState,
		setLeaveManagementState
	} from './management/leave-management-state.svelte';
	import { todayDateInTimeZone } from './shared/attendance-date';
	import { attendanceText } from './text';
	import {
		setWorkStatusState,
		WorkStatusState
	} from './work-status/work-status-state.svelte';

	let { children, cacheScope }: { children: import('svelte').Snippet; cacheScope: string } = $props();
	let isDisposed = false;

	const text = createPageText(attendanceText);
	const attendance = untrack(() => new AttendanceState(text.loadFailed, cacheScope));
	const attendanceView = new AttendanceViewState(ensureSelectedAdminView);
	const workStatus = untrack(() => new WorkStatusState(cacheScope));
	const isAdmin = $derived(attendance.summary?.isAdmin ?? false);

	async function refreshAfterEmployeeLeaveMutation(): Promise<void> {
		clearAdminView('approvals');
		clearAdminView('leaveManagement');
		await Promise.all([attendance.load(), refreshSelectedAdminView()]);
	}

	async function refreshAfterApprovalMutation(): Promise<void> {
		clearAdminView('leaveManagement');
		await Promise.all([
			attendance.load(),
			employeeLeave.load()
		]);
	}

	async function refreshAfterManagementMutation(): Promise<void> {
		clearAdminView('approvals');
		await Promise.all([
			attendance.load(),
			employeeLeave.load()
		]);
	}

	const employeeLeave = new EmployeeLeaveState(text.leave, refreshAfterEmployeeLeaveMutation);
	const leaveApproval = new LeaveApprovalState(text.approval, refreshAfterApprovalMutation);
	const leaveManagement = new LeaveManagementState(
		text.management,
		refreshAfterManagementMutation
	);
	const handWritten = new HandWrittenState(text.handWritten, () => attendance.load());
	setAttendanceState(attendance);
	setEmployeeLeaveState(employeeLeave);
	setAttendanceViewState(attendanceView);
	setLeaveApprovalState(leaveApproval);
	setLeaveManagementState(leaveManagement);
	setHandWrittenState(handWritten);
	setWorkStatusState(workStatus);

	const pendingAdminViews = new Map<string, Promise<void>>();

	function clearAdminView(view: AttendanceWorkspaceView): void {
		pendingAdminViews.delete(view);
		if (view === 'approvals') leaveApproval.clear();
		if (view === 'leaveManagement') leaveManagement.clear();
		if (view === 'handWritten') handWritten.clear();
	}

	async function ensureSelectedAdminView(view: AttendanceWorkspaceView = attendanceView.selected): Promise<void> {
		if (!isAdmin) return;
		if (view === 'approvals' && leaveApproval.inbox) return;
		if (view === 'leaveManagement' && leaveManagement.payload) return;
		if (view === 'handWritten' && handWritten.dayRange.from) return;
		if (pendingAdminViews.has(view)) return pendingAdminViews.get(view);
		const reading = refreshSelectedAdminView(view).finally(() => {
			if (pendingAdminViews.get(view) === reading) pendingAdminViews.delete(view);
		});
		pendingAdminViews.set(view, reading);
		await reading;
	}

	function refreshServerClock(): void {
		void attendance.refreshServerClock();
	}

	function refreshVisibleServerClock(): void {
		if (document.visibilityState !== 'visible') return;
		refreshServerClock();
	}

	async function refreshSelectedAdminView(selectedView: AttendanceWorkspaceView = attendanceView.selected): Promise<void> {
		if (!isAdmin) return;
		if (selectedView === 'approvals') await leaveApproval.load();
		if (selectedView === 'leaveManagement') await leaveManagement.load();
		if (selectedView === 'handWritten') await handWritten.load(todayDateInTimeZone(attendance.summary?.timeZone));
	}

	onMount(() => {
		myAttendanceToday.setClockEventHandler((event) => workStatus.applyAttendanceEvent(event));
		void attendance.load();
		void employeeLeave.load();
		const releaseRefresh = pageActions.setRefresh(async () => {
			await Promise.all([attendance.load(), employeeLeave.load(), refreshSelectedAdminView()]);
		});
		window.addEventListener('focus', refreshServerClock);
		window.addEventListener('pageshow', refreshServerClock);
		document.addEventListener('visibilitychange', refreshVisibleServerClock);
		return () => {
			isDisposed = true;
			attendance.dispose();
			myAttendanceToday.setClockEventHandler(undefined);
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
		myAttendanceToday.adoptSummary(attendance.currentMonthSummary);
	});

	$effect(() => {
		const summary = attendance.summary;
		const period = attendance.chartMode;
		const selectedMonth = attendance.selectedMonth;
		if (!summary) return;
		const today = todayDateInTimeZone(summary.timeZone);
		const anchor =
			selectedMonth && !today.startsWith(selectedMonth) ? `${selectedMonth}-01` : today;
		untrack(() => void workStatus.load(period, anchor, summary));
	});

	$effect(() => {
		if (isAdmin) return;
		untrack(() => leaveApproval.clear());
		untrack(() => leaveManagement.clear());
		untrack(() => handWritten.clear());
	});

	$effect(() => {
		const selectedView = attendanceView.selected;
		if (!isAdmin) return;
		untrack(() => void ensureSelectedAdminView(selectedView));
	});

	$effect(() => {
		const selectedView = attendanceView.selected;
		if (!isAdmin) return;
		const nextView = selectedView === 'approvals' ? 'leaveManagement'
			: selectedView === 'leaveManagement' ? 'handWritten'
			: selectedView === 'status' || selectedView === 'tools' ? 'approvals' : null;
		if (!nextView) return;
		const prepare = () => {
			if (!isDisposed && document.visibilityState === 'visible') void ensureSelectedAdminView(nextView);
		};
		if ('requestIdleCallback' in window) {
			const job = window.requestIdleCallback(prepare, { timeout: 200 });
			return () => window.cancelIdleCallback(job);
		}
		const timer = setTimeout(prepare, 150);
		return () => clearTimeout(timer);
	});
</script>

<div class="flex min-h-0 min-w-0 flex-1">
	<AttendanceSidebar />

	<div class="min-w-0 flex-1 overflow-y-auto p-3 md:p-6">
		{@render children()}
	</div>
</div>
