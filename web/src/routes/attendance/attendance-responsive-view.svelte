<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Tabs from '$lib/components/ui/tabs';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceLoadingSkeleton from './attendance-loading-skeleton.svelte';
	import { getAttendanceState } from './attendance-context.svelte';
	import { getAttendanceViewState } from './attendance-view-state.svelte';
	import LeaveApprovalView from './approval/leave-approval-view.svelte';
	import { getLeaveApprovalState } from './approval/leave-approval-state.svelte';
	import LeaveHistoryView from './leave/leave-history-view.svelte';
	import HandWrittenView from './hand-written/hand-written-view.svelte';
	import LeaveManagementView from './management/leave-management-view.svelte';
	import PersonalToolsPanel from './personal/personal-tools-panel.svelte';
	import TeamView from './team/team-view.svelte';
	import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const attendanceView = getAttendanceViewState();
	const leaveApproval = getLeaveApprovalState();
	const isMobile = new IsMobile();

	$effect(() => {
		if (
			!attendance.summary?.isAdmin &&
			(attendanceView.selected === 'approvals' ||
				attendanceView.selected === 'leaveManagement' ||
				attendanceView.selected === 'handWritten')
		) {
			attendanceView.select(isMobile.current ? 'tools' : 'status');
			return;
		}
		if (!isMobile.current && attendanceView.selected === 'tools') {
			attendanceView.select('status');
		}
	});
</script>

{#if !attendance.summary}
	<AttendanceLoadingSkeleton />
{:else}
	<Tabs.Root
		bind:value={attendanceView.selected}
		class="min-h-0 min-w-0 flex-1 gap-3 max-sm:pb-[calc(var(--app-mobile-nav-bottom)+var(--app-mobile-nav-height)+0.75rem)]"
	>
	<Tabs.List
		class="inline-flex h-9 w-full max-w-full self-start justify-start overflow-x-auto rounded-full border-0 bg-muted p-1 md:hidden"
	>
		<Tabs.Trigger
			value="tools"
			class="h-7 flex-none rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
		>
			{text.mobileToolsView}
		</Tabs.Trigger>
		<Tabs.Trigger
			value="status"
			class="h-7 flex-none rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
		>
			{text.mobileStatusView}
		</Tabs.Trigger>
		<Tabs.Trigger
			value="leaveHistory"
			class="h-7 flex-none gap-1.5 rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
		>
			{text.leave.historyTab}
		</Tabs.Trigger>
		{#if attendance.summary?.isAdmin}
			<Tabs.Trigger
				value="approvals"
				class="h-7 flex-none gap-1.5 rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
			>
				{text.approval.mobileTab}
				<Badge variant="secondary">{leaveApproval.inbox?.pendingCount ?? 0}</Badge>
			</Tabs.Trigger>
			<Tabs.Trigger
				value="leaveManagement"
				class="h-7 flex-none rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
			>
				{text.management.mobileTab}
			</Tabs.Trigger>
			<Tabs.Trigger
				value="handWritten"
				class="h-7 flex-none rounded-full px-3 text-xs font-semibold text-muted-foreground data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
			>
				{text.handWritten.mobileTab}
			</Tabs.Trigger>
		{/if}
	</Tabs.List>
	<Tabs.Content value="status" class="min-h-0 min-w-0">
		<TeamView />
	</Tabs.Content>
	<Tabs.Content value="leaveHistory" class="min-h-0 min-w-0">
		<LeaveHistoryView />
	</Tabs.Content>
	<Tabs.Content
		value="tools"
		class="min-h-[calc(100vh-9rem)] min-w-0 overflow-auto"
		data-testid="mobile-attendance-tools-view"
	>
		{#if isMobile.current}
			<PersonalToolsPanel
				containerClass="pb-4 [&>[data-slot=card]]:border [&>[data-slot=card]]:border-border [&>[data-slot=card]]:ring-0"
			/>
		{/if}
	</Tabs.Content>
	{#if attendance.summary?.isAdmin}
		<Tabs.Content value="approvals" class="min-h-0 min-w-0">
			<LeaveApprovalView />
		</Tabs.Content>
		<Tabs.Content value="leaveManagement" class="min-h-0 min-w-0">
			<LeaveManagementView />
		</Tabs.Content>
		<Tabs.Content value="handWritten" class="min-h-0 min-w-0">
			<HandWrittenView />
		</Tabs.Content>
	{/if}
	</Tabs.Root>
{/if}
