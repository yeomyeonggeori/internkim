<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import SettingsMobileList from './settings-mobile-list.svelte';
	import { settingsRole } from './settings-role.svelte';
	import { firstSettingsSectionID, settingsAccessOf, settingsPath } from './settings-sections';
	import { myCredits } from './usage/my-credits.svelte';

	const isMobile = new IsMobile();
	const role = settingsRole();
	const credits = myCredits();
	const access = $derived(settingsAccessOf(role, credits, isSupabaseConfigured()));

	$effect(() => {
		if (isMobile.current) return;
		void goto(settingsPath(page.url.pathname, firstSettingsSectionID), { replaceState: true });
	});
</script>

<SettingsMobileList {access} />
