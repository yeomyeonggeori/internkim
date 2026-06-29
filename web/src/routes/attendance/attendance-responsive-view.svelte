<script lang="ts">
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PersonalToolsPanel from './personal/personal-tools-panel.svelte';
	import TeamView from './team/team-view.svelte';
	import { attendanceText } from './text';

	type MobileAttendanceViewTab = 'status' | 'tools';

	const text = createPageText(attendanceText);
	const isMobile = new IsMobile();
	let selectedTab = $state<MobileAttendanceViewTab>('tools');

	$effect(() => {
		if (!isMobile.current) selectedTab = 'status';
	});
</script>

<Tabs.Root bind:value={selectedTab} class="min-h-0 min-w-0 flex-1 gap-3">
	<Tabs.List class="inline-flex h-9 w-fit self-start rounded-full border-0 bg-muted p-1 md:hidden">
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
