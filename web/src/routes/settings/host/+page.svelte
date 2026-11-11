<script lang="ts">
	import { page } from '$app/state';
	import { buttonVariants } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CompanyHost from '../company-host.svelte';
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { settingsRole } from '../settings-role.svelte';
	import { settingsPath } from '../settings-sections';
	import { hostSetupText } from '../setup/text';

	const setupText = createPageText(hostSetupText);
	const role = settingsRole();
</script>

<SettingsSectionPage section="host">
	{#if role.isAdmin}
		<a href={settingsPath(page.url.pathname, 'setup')} class={buttonVariants({ variant: 'outline', class: 'w-fit' })}>{setupText.title}</a>
	{/if}
	<CompanyHost isAdmin={role.isAdmin} />
</SettingsSectionPage>
