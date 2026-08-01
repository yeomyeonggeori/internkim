<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import ChartNoAxesCombinedIcon from '@lucide/svelte/icons/chart-no-axes-combined';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LayoutDashboardIcon from '@lucide/svelte/icons/layout-dashboard';
	import HistoryIcon from '@lucide/svelte/icons/history';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { getAttendanceState } from './attendance-context.svelte';
	import { getAttendanceViewState } from './attendance-view-state.svelte';
	import { getLeaveApprovalState } from './approval/leave-approval-state.svelte';
	import { getEmployeeLeaveState } from './leave/employee-leave-state.svelte';
	import PersonalToolsPanel from './personal/personal-tools-panel.svelte';
	import { timeZoneDisplayLabel } from './shared/attendance-date';
	import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const attendanceView = getAttendanceViewState();
	const leaveApproval = getLeaveApprovalState();
	const employeeLeave = getEmployeeLeaveState();
	const needsChangesCount = $derived(
		employeeLeave.payload?.requests.filter((request) => request.status === 'needsChanges').length ?? 0
	);

	async function refresh(): Promise<void> {
		const requests: Promise<void>[] = [attendance.load()];
		if (attendance.summary?.isAdmin) requests.push(leaveApproval.load());
		await Promise.all(requests);
	}
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<ClipboardCheckIcon class="size-4 text-muted-foreground" />
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-medium">{text.title}</p>
			<p class="truncate text-xs text-muted-foreground">
				{attendance.summary ? timeZoneDisplayLabel(attendance.summary.timeZone) : '-'}
			</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={refresh}>
			<RefreshCwIcon class={attendance.isLoading || leaveApproval.isLoading ? 'animate-spin' : ''} />
		</Button>
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto" data-testid="attendance-sidebar-scroll">
		<nav class="space-y-1 px-4 pt-4" aria-label={text.title}>
			<Button
				variant={attendanceView.selected === 'status' ? 'secondary' : 'ghost'}
				class="w-full justify-start"
				onclick={() => attendanceView.select('status')}
			>
				<LayoutDashboardIcon />
				{text.approval.statusNavigation}
			</Button>
			{#if attendance.summary?.isAdmin}
				<Button
					variant={attendanceView.selected === 'employeeWorkStatus' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('employeeWorkStatus')}
					data-testid="employee-work-status-navigation"
				>
					<ChartNoAxesCombinedIcon />
					{text.workStatus.employeeNavigation}
				</Button>
			{/if}
			<Button
				variant={attendanceView.selected === 'leaveHistory' ? 'secondary' : 'ghost'}
				class="w-full justify-start"
				onclick={() => attendanceView.select('leaveHistory')}
				data-testid="leave-history-navigation"
			>
				<HistoryIcon />
				<span class="flex-1 text-left">{text.leave.historyTab}</span>
				{#if needsChangesCount > 0}
					<Badge variant="secondary" data-testid="leave-history-needs-changes-count">
						{needsChangesCount}
					</Badge>
				{/if}
			</Button>
			{#if attendance.summary?.isAdmin}
				<Button
					variant={attendanceView.selected === 'approvals' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('approvals')}
					data-testid="leave-approval-navigation"
				>
					<InboxIcon />
					<span class="flex-1 text-left">{text.approval.pendingNavigation}</span>
					<Badge variant="secondary">{leaveApproval.inbox?.pendingCount ?? 0}</Badge>
				</Button>
				<Button
					variant={attendanceView.selected === 'leaveManagement' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('leaveManagement')}
					data-testid="leave-management-navigation"
				>
					<UsersRoundIcon />
					{text.management.navigation}
				</Button>
			{/if}
		</nav>

		<PersonalToolsPanel containerClass="p-4 pt-3" />
	</div>
</aside>
