<script lang="ts">
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { isSupabaseConfigured } from '$lib/supabase-session';
	import CompanyMembers from '../company-members.svelte';
	import Circles from '../circles.svelte';
	import CompanyFields from '../company-fields.svelte';
</script>

<SettingsSectionPage section="members">
	<CompanyMembers />
	{#if isSupabaseConfigured()}
		<Circles />
		<CompanyFields />
	{/if}
</SettingsSectionPage>
