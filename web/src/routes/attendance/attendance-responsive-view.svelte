<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import HistoryIcon from '@lucide/svelte/icons/history';
	import PalmTreeIcon from '@lucide/svelte/icons/palmtree';
	import EraserIcon from '@lucide/svelte/icons/eraser';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import * as Tabs from '$lib/components/ui/tabs';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceLoadingSkeleton from './attendance-loading-skeleton.svelte';
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
	const attendanceView = getAttendanceViewState();
	const leaveApproval = getLeaveApprovalState();
	const isMobile = new IsMobile();

	$effect(() => {
		if (
			!myAttendanceToday.summary?.isAdmin &&
			(attendanceView.selected === 'approvals' ||
				attendanceView.selected === 'leaveManagement' ||
				attendanceView.selected === 'handWritten')
		) {
			attendanceView.select('status');
			return;
		}

	});
</script>

{#if !myAttendanceToday.summary}
	<AttendanceLoadingSkeleton />
	{#if myAttendanceToday.loadFailure}
		<p role="alert" class="mt-3 text-sm text-destructive">{myAttendanceToday.loadFailure}</p>
		<Button variant="outline" size="sm" onclick={() => myAttendanceToday.refresh()}>{text.refresh}</Button>
	{/if}
{:else}
	<Tabs.Root
		bind:value={attendanceView.selected}
		class="min-h-0 min-w-0 flex-1 gap-3 max-sm:pb-[calc(var(--app-mobile-nav-bottom)+var(--app-mobile-nav-height)+0.75rem)]"
	>
    <nav class="flex items-center gap-2 md:hidden" aria-label={text.title}>
        <Button size="sm" variant={attendanceView.selected === 'status' ? 'secondary' : 'ghost'} onclick={() => attendanceView.select('status')}><HistoryIcon class="size-4" />{text.navigation.status}</Button>
        <Button size="sm" variant={attendanceView.selected === 'leaveHistory' ? 'secondary' : 'ghost'} onclick={() => attendanceView.select('leaveHistory')}><PalmTreeIcon class="size-4" />{text.navigation.mine}</Button>
        {#if myAttendanceToday.summary?.isAdmin}
            <DropdownMenu.Root>
                <DropdownMenu.Trigger class="ml-auto inline-flex items-center gap-1 rounded-md px-3 py-2 text-sm">{text.navigation.management}<ChevronDownIcon class="size-3.5" /></DropdownMenu.Trigger>
                <DropdownMenu.Content align="end">
                    <DropdownMenu.Item onclick={() => attendanceView.select('approvals')}><InboxIcon class="size-4" />{text.approval.pendingNavigation}{#if leaveApproval.inbox}<Badge variant="secondary">{leaveApproval.inbox.pendingCount}</Badge>{/if}</DropdownMenu.Item>
                    <DropdownMenu.Item onclick={() => attendanceView.select('leaveManagement')}><UsersRoundIcon class="size-4" />{text.management.navigation}</DropdownMenu.Item>
                    <DropdownMenu.Item onclick={() => attendanceView.select('handWritten')}><EraserIcon class="size-4" />{text.handWritten.navigation}</DropdownMenu.Item>
                </DropdownMenu.Content>
            </DropdownMenu.Root>
        {/if}
    </nav>
    {#if ['approvals','leaveManagement','handWritten'].includes(attendanceView.selected)}
        <p class="text-sm font-medium md:hidden">{attendanceView.selected === 'approvals' ? text.approval.pendingNavigation : attendanceView.selected === 'leaveManagement' ? text.management.navigation : text.handWritten.navigation}</p>
    {/if}

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
			<PersonalToolsPanel
				containerClass="pb-4 [&>[data-slot=card]]:border [&>[data-slot=card]]:border-border [&>[data-slot=card]]:ring-0"
			/>
	</Tabs.Content>
	{#if myAttendanceToday.summary?.isAdmin}
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
