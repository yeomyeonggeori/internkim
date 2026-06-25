<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import TeamView from './team/team-view.svelte';
	import { attendanceText } from './text';

	type MobileAttendanceViewTab = 'status' | 'tools';

	const text = createPageText(attendanceText);
	const isMobile = new IsMobile();
	let selectedTab = $state<MobileAttendanceViewTab>('status');

	$effect(() => {
		if (!isMobile.current) selectedTab = 'status';
	});
</script>

<Tabs.Root bind:value={selectedTab} class="min-h-0 min-w-0 flex-1 gap-3">
	<Tabs.List class="grid h-10 w-full grid-cols-2 rounded-md border bg-muted/50 p-1 md:hidden">
		<Tabs.Trigger value="status">{text.mobileStatusView}</Tabs.Trigger>
		<Tabs.Trigger value="tools">{text.mobileToolsView}</Tabs.Trigger>
	</Tabs.List>
	<Tabs.Content value="status" class="min-h-0 min-w-0">
		<TeamView />
	</Tabs.Content>
	<Tabs.Content value="tools" class="min-h-[calc(100vh-9rem)] min-w-0" data-testid="mobile-attendance-tools-view"></Tabs.Content>
</Tabs.Root>
