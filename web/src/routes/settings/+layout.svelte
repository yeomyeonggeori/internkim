<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { routePathOf } from '$lib/company-path';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import { onMount, type Snippet } from 'svelte';
	import SettingsSidebar from './settings-sidebar.svelte';
	import { createSettingsReadAhead } from './settings-reads';
	import { provideSettingsRole } from './settings-role.svelte';
	import { settingsAccessOf, settingsPath, settingsSectionAvailability, settingsSectionIDOf } from './settings-sections';
	import { companySettingsText } from './text';
	import { provideMyCredits } from './usage/my-credits.svelte';

	let { children }: { children: Snippet } = $props();

	const text = createPageText(companySettingsText);
	const role = provideSettingsRole();
	const credits = provideMyCredits();
	const access = $derived(settingsAccessOf(role, credits, isSupabaseConfigured()));
	const sectionID = $derived(settingsSectionIDOf(routePathOf(page.url.pathname)));
	const availability = $derived(sectionID ? settingsSectionAvailability(sectionID, access) : 'shown');
	const readAhead = createSettingsReadAhead();

	$effect(() => readAhead(access));

	onMount(() => {
		void role.load();
		if (isSupabaseConfigured()) void credits.load();
	});

	$effect(() => {
		if (availability !== 'hidden') return;
		void goto(settingsPath(page.url.pathname), { replaceState: true });
	});
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<div class="flex h-full min-h-0 w-full flex-1">
	<SettingsSidebar {access} currentSectionID={sectionID} />
	<div class="flex min-h-0 min-w-0 flex-1 flex-col">
		{#if availability === 'shown'}
			{@render children()}
		{/if}
	</div>
</div>
