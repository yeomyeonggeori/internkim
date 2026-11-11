<script lang="ts">
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import PersonalAccessTokens from '../personal-access-tokens.svelte';
	import ConnectedApps from '../connected-apps.svelte';
</script>

<SettingsSectionPage section="developer">
	<PersonalAccessTokens />
	{#if isSupabaseConfigured()}
		<ConnectedApps />
	{/if}
</SettingsSectionPage>
