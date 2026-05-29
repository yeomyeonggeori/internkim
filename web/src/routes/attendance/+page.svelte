<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import * as Tabs from '$lib/components/ui/tabs';
	import { getAttendanceState, type AttendanceTab } from './attendance-context.svelte';
	import TeamView from './team/team-view.svelte';
	import PersonalView from './personal/personal-view.svelte';
	import { attendanceText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	onMount(() => {
		const fromUrl = new URLSearchParams(window.location.search).get('tab');
		if (fromUrl === 'team' || fromUrl === 'personal') {
			attendance.setTab(fromUrl);
		}
	});

	function setTab(value: string) {
		const next = value as AttendanceTab;
		attendance.setTab(next);
		const params = new URLSearchParams($page.url.searchParams);
		params.set('tab', next);
		goto(`?${params.toString()}`, { replaceState: true, keepFocus: true, noScroll: true });
	}

	const isTeamBlocked = $derived(
		!!attendance.summary && !attendance.summary.isAdmin && attendance.summary.teamViewBlocked
	);

	const tabTriggerClass =
		'rounded-full px-4 data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:hover:text-primary-foreground dark:data-[state=active]:bg-primary dark:data-[state=active]:text-primary-foreground dark:data-[state=active]:hover:text-primary-foreground dark:data-[state=active]:border-transparent';
</script>

<Tabs.Root value={attendance.tab} onValueChange={setTab} class="flex flex-col gap-4">
	<Tabs.List class="rounded-full">
		<Tabs.Trigger value="team" disabled={isTeamBlocked} class={tabTriggerClass}>{text.tabTeam}</Tabs.Trigger>
		<Tabs.Trigger value="personal" class={tabTriggerClass}>{text.tabPersonal}</Tabs.Trigger>
	</Tabs.List>
	<Tabs.Content value="team">
		{#if isTeamBlocked}
			<p class="text-sm text-muted-foreground">{text.teamBlocked}</p>
		{:else}
			<TeamView />
		{/if}
	</Tabs.Content>
	<Tabs.Content value="personal">
		<PersonalView />
	</Tabs.Content>
</Tabs.Root>
