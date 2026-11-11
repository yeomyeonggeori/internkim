<script lang="ts">
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import SignInPasskeys from '../sign-in-passkeys.svelte';
	import SignInPassword from '../sign-in-password.svelte';
</script>

<SettingsSectionPage section="security">
	<SignInPasskeys />
	{#if isSupabaseConfigured()}
		<SignInPassword />
	{/if}
</SettingsSectionPage>
