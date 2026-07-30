<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
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

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.credentials.title}</Card.Title>
		<Card.Description>{text.credentials.description}</Card.Description>
		<Card.Action>
			<Badge variant={openRouterProvider()?.configured ? 'secondary' : 'outline'}>
				{openRouterProvider()?.configured ? text.credentials.configured : text.credentials.missing}
			</Badge>
		</Card.Action>
	</Card.Header>
	<Card.Content>
		<Field.Group>
			<Item.Root variant="outline">
				<Item.Content>
					<Item.Title>OpenRouter</Item.Title>
					<Item.Description>
						{#if openRouterProvider()?.fingerprint}
							{openRouterProvider()?.fingerprint}
						{:else if isLoadingCredentials}
							{text.credentials.loading}
						{:else}
							{text.credentials.noKey}
						{/if}
					</Item.Description>
				</Item.Content>
				{#if openRouterProvider()?.configured}
					<Item.Actions>
						<Button variant="outline" size="sm" disabled={isSavingCredential} onclick={deleteOpenRouterKey}>
							{text.credentials.delete}
						</Button>
					</Item.Actions>
				{/if}
			</Item.Root>
			<form
				onsubmit={(event) => {
					event.preventDefault();
					saveOpenRouterKey();
				}}
			>
				<Field.Field>
					<Field.Label for="openrouter-api-key">{text.credentials.openRouterApiKeyPlaceholder}</Field.Label>
					<div class="flex flex-wrap items-center gap-2">
						<Input
							id="openrouter-api-key"
							class="min-w-64 flex-1"
							bind:value={openRouterApiKey}
							type="password"
							autocomplete="new-password"
						/>
						<Button type="submit" disabled={!isDeviceReachable || isSavingCredential || !openRouterApiKey.trim()}>
							{#if isSavingCredential}
								<LoaderIcon class="size-4 animate-spin" />
							{/if}
							{text.credentials.save}
						</Button>
					</div>
					<Field.Description>{text.credentials.notice}</Field.Description>
				</Field.Field>
			</form>
			{#if credentialErrorMessage}
				<Field.Error>{credentialErrorMessage}</Field.Error>
			{/if}
		</Field.Group>
	</Card.Content>
</Card.Root>
