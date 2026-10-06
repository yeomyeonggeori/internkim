<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import HistoryIcon from '@lucide/svelte/icons/history';
	import PalmTreeIcon from '@lucide/svelte/icons/palmtree';
	import EraserIcon from '@lucide/svelte/icons/eraser';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { getAttendanceViewState } from './attendance-view-state.svelte';
	import { getLeaveApprovalState } from './approval/leave-approval-state.svelte';
		import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendanceView = getAttendanceViewState();
	const leaveApproval = getLeaveApprovalState();
	const isSidebarHidden = new IsMobile(768);
</script>

<aside class="flex w-52 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="min-h-0 flex-1 overflow-y-auto" data-testid="attendance-sidebar-scroll">
		<nav class="space-y-1 px-4 pt-4" aria-label={text.title}>
			<Button
				variant={attendanceView.selected === 'status' ? 'secondary' : 'ghost'}
				class="w-full justify-start"
				onclick={() => attendanceView.select('status')}
			>
				<HistoryIcon />
				{text.navigation.status}
			</Button>
			<Button
				variant={attendanceView.selected === 'leaveHistory' ? 'secondary' : 'ghost'}
				class="w-full justify-start"
				onclick={() => attendanceView.select('leaveHistory')}
				data-testid="leave-history-navigation"
			>
				<PalmTreeIcon />
				<span class="flex-1 text-left">{text.navigation.mine}</span>
			</Button>

			{#if myAttendanceToday.summary?.isAdmin}
                <div class="mt-4 border-t pt-4 pb-1 text-xs font-medium text-muted-foreground">{text.navigation.admin}</div>
				<Button
					variant={attendanceView.selected === 'approvals' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('approvals')}
				onpointerenter={() => attendanceView.prefetch('approvals')}
				onfocus={() => attendanceView.prefetch('approvals')}
				ontouchstart={() => attendanceView.prefetch('approvals')}
					data-testid="leave-approval-navigation"
				>
					<InboxIcon />
					<span class="flex-1 text-left">{text.approval.pendingNavigation}</span>
					{#if leaveApproval.inbox}
						<Badge variant="secondary">{leaveApproval.inbox.pendingCount}</Badge>
					{/if}
				</Button>
				<Button
					variant={attendanceView.selected === 'leaveManagement' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('leaveManagement')}
				onpointerenter={() => attendanceView.prefetch('leaveManagement')}
				onfocus={() => attendanceView.prefetch('leaveManagement')}
				ontouchstart={() => attendanceView.prefetch('leaveManagement')}
					data-testid="leave-management-navigation"
				>
					<UsersRoundIcon />
					{text.management.navigation}
				</Button>
				<Button
					variant={attendanceView.selected === 'handWritten' ? 'secondary' : 'ghost'}
					class="w-full justify-start"
					onclick={() => attendanceView.select('handWritten')}
				onpointerenter={() => attendanceView.prefetch('handWritten')}
				onfocus={() => attendanceView.prefetch('handWritten')}
				ontouchstart={() => attendanceView.prefetch('handWritten')}
					data-testid="hand-written-navigation"
				>
					<EraserIcon />
					{text.handWritten.navigation}
				</Button>
			{/if}
		</nav>


	</div>
</aside>
