<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import AttendanceLoadingSkeleton from './attendance-loading-skeleton.svelte';
	import { getAttendanceState } from './attendance-context.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PersonalToolsPanel from './personal/personal-tools-panel.svelte';
	import TeamView from './team/team-view.svelte';
	import { attendanceText } from './text';

	type MobileAttendanceViewTab = 'status' | 'tools';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const isMobile = new IsMobile();
	let selectedTab = $state<MobileAttendanceViewTab>('tools');

	$effect(() => {
		if (!isMobile.current) selectedTab = 'status';
	});
</script>

{#if !attendance.summary}
	<AttendanceLoadingSkeleton />
{:else}
	<Tabs.Root bind:value={selectedTab} class="min-h-0 min-w-0 flex-1 gap-3">
		<Tabs.List class="self-start md:hidden">
			<Tabs.Trigger value="tools">{text.mobileToolsView}</Tabs.Trigger>
			<Tabs.Trigger value="status">{text.mobileStatusView}</Tabs.Trigger>
		</Tabs.List>
		<Tabs.Content value="status" class="min-h-0 min-w-0">
			<TeamView />
		</Tabs.Content>
		<Tabs.Content value="tools" class="min-h-[calc(100vh-9rem)] min-w-0 overflow-auto" data-testid="mobile-attendance-tools-view">
			{#if isMobile.current}
				<PersonalToolsPanel containerClass="pb-4 [&>[data-slot=card]]:border [&>[data-slot=card]]:border-border [&>[data-slot=card]]:ring-0" />
			{/if}
		</Tabs.Content>
	</Tabs.Root>
{/if}
