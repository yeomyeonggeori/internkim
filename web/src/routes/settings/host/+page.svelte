<script lang="ts">
	import type { HostSetupStatus } from '$lib/company/host-setup';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CompanyHost from '../company-host.svelte';
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { settingsRole } from '../settings-role.svelte';
	import CompanyComputerCard from '../setup/company-computer-card.svelte';
	import CompanyComputerCardSkeleton from '../setup/company-computer-card-skeleton.svelte';
	import FirstMessage from '../setup/first-message.svelte';
	import { fetchHostSetupStatus } from '../setup/host-setup-client';
	import { hostSetupText } from '../setup/text';

	const setupText = createPageText(hostSetupText);
	const role = settingsRole();
	let status = $state<HostSetupStatus | null>(null);
	let isLoading = $state(true);
	let errorMessage = $state('');

	$effect(() => {
		if (role.isAdmin) void loadStatus();
	});

	async function loadStatus() {
		try {
			status = await fetchHostSetupStatus();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : setupText.failed;
		} finally {
			isLoading = false;
		}
	}
</script>

<SettingsSectionPage section="host">
	{#if role.isAdmin}
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		{#if status}
			<CompanyComputerCard bind:status bind:errorMessage />
		{:else if isLoading}
			<div role="status" aria-label={setupText.loading} aria-busy="true"><CompanyComputerCardSkeleton /></div>
		{/if}
	{/if}
	<CompanyHost isAdmin={role.isAdmin} />
	{#if role.isAdmin && status}
		<FirstMessage companySlug={status.company.slug} bind:errorMessage />
	{/if}
</SettingsSectionPage>
