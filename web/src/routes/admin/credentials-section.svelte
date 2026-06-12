<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { apiErrorMessage, deleteOpenRouterCredential, fetchCredentialProviders, saveOpenRouterCredential } from './admin-api';
	import type { AdminPageText, CredentialProviderStatus } from './admin-types';

	type CredentialsSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: CredentialsSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let credentialProviders = $state<CredentialProviderStatus[]>([]);
	let openRouterApiKey = $state('');
	let credentialErrorMessage = $state('');
	let isLoadingCredentials = $state(false);
	let isSavingCredential = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadCredentials();
	});

	function openRouterProvider() {
		return credentialProviders.find((provider) => provider.provider === 'openrouter');
	}

	function replaceCredentialProvider(provider: CredentialProviderStatus) {
		credentialProviders = [provider, ...credentialProviders.filter((candidate) => candidate.provider !== provider.provider)];
	}

	async function loadCredentials() {
		if (!adminBaseURL) return;

		isLoadingCredentials = true;
		credentialErrorMessage = '';
		try {
			const response = await fetchCredentialProviders(adminBaseURL, text.messages.credentialsLoadError);
			credentialProviders = response.providers ?? [];
		} catch {
			credentialErrorMessage = text.messages.credentialsLoadError;
		} finally {
			isLoadingCredentials = false;
		}
	}

	async function saveOpenRouterKey() {
		const apiKey = openRouterApiKey.trim();
		if (!adminBaseURL || !apiKey) return;

		isSavingCredential = true;
		credentialErrorMessage = '';
		try {
			replaceCredentialProvider(await saveOpenRouterCredential(adminBaseURL, apiKey, text.messages.openRouterSaveError));
			openRouterApiKey = '';
		} catch (error) {
			credentialErrorMessage = apiErrorMessage(error, text.messages.openRouterSaveError);
		} finally {
			isSavingCredential = false;
		}
	}

	async function deleteOpenRouterKey() {
		if (!adminBaseURL) return;

		isSavingCredential = true;
		credentialErrorMessage = '';
		try {
			replaceCredentialProvider(await deleteOpenRouterCredential(adminBaseURL, text.messages.openRouterDeleteError));
		} catch (error) {
			credentialErrorMessage = apiErrorMessage(error, text.messages.openRouterDeleteError);
		} finally {
			isSavingCredential = false;
		}
	}
</script>

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="text-sm font-semibold">{text.credentials.title}</h3>
			<p class="text-muted-foreground mt-1 text-sm">
				{text.credentials.description}
			</p>
		</div>
		<Badge variant={openRouterProvider()?.configured ? 'secondary' : 'outline'}>
			{openRouterProvider()?.configured ? text.credentials.configured : text.credentials.missing}
		</Badge>
	</div>
	<div class="grid gap-3">
		<div class="rounded-md bg-muted/30 p-3">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<p class="text-sm font-medium">OpenRouter</p>
					<p class="text-muted-foreground mt-1 text-xs">
						{#if openRouterProvider()?.fingerprint}
							{openRouterProvider()?.fingerprint}
						{:else if isLoadingCredentials}
							{text.credentials.loading}
						{:else}
							{text.credentials.noKey}
						{/if}
					</p>
				</div>
				{#if openRouterProvider()?.configured}
					<Button variant="ghost" size="sm" disabled={isSavingCredential} onclick={deleteOpenRouterKey}>
						{text.credentials.delete}
					</Button>
				{/if}
			</div>
		</div>
		<form
			class="grid gap-2 sm:grid-cols-[1fr_auto]"
			onsubmit={(event) => {
				event.preventDefault();
				saveOpenRouterKey();
			}}
		>
			<Input bind:value={openRouterApiKey} type="password" placeholder={text.credentials.openRouterApiKeyPlaceholder} autocomplete="new-password" />
			<Button type="submit" disabled={!isDeviceReachable || isSavingCredential || !openRouterApiKey.trim()} class="gap-2">
				{#if isSavingCredential}
					<LoaderIcon class="size-4 animate-spin" />
				{/if}
				{text.credentials.save}
			</Button>
		</form>
		<p class="text-muted-foreground text-xs">{text.credentials.notice}</p>
	</div>
	{#if credentialErrorMessage}
		<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
			{credentialErrorMessage}
		</p>
	{/if}
</div>
