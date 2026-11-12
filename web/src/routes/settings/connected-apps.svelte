<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import SettingsListLoading from './settings-list-loading.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import { disconnectApp, type ConnectedApp } from '$lib/connected-apps';
	import { connectedAppsRead } from './settings-reads';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);

	const apps = $derived(connectedAppsRead.value ?? []);
	let isLoading = $state(connectedAppsRead.value === undefined);
	let hasLoadError = $state(false);
	let disconnectingID = $state('');

	async function load(isAfterChange = false) {
		isLoading = true;
		hasLoadError = false;
		try {
			await (isAfterChange ? connectedAppsRead.refresh() : connectedAppsRead.read());
		} catch {
			hasLoadError = true;
			toast.error(text.connectedAppsLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function disconnect(app: ConnectedApp) {
		disconnectingID = app.clientID;
		try {
			await disconnectApp(app.clientID);
			toast.success(text.appDisconnected);
			await load(true);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.connectedAppsLoadFailed);
		} finally {
			disconnectingID = '';
		}
	}

	onMount(load);
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.connectedApps}</Card.Title>
		<Card.Description>{text.connectedAppsDescription}</Card.Description>
	</Card.Header>
	{#if isLoading && connectedAppsRead.value === undefined}
		<Card.Content><SettingsListLoading label={text.connectedApps} actionLabel={text.disconnectApp} /></Card.Content>
	{:else}
		<Card.Content>
			{#if hasLoadError}<p role="alert" class="text-sm text-destructive">{text.connectedAppsLoadFailed}</p>{/if}
			{#if apps.length === 0 && !hasLoadError}
				<Empty.Root>
					<Empty.Header><Empty.Title>{text.noConnectedApps}</Empty.Title></Empty.Header>
				</Empty.Root>
			{:else}
				<ul class="grid gap-2">
					{#each apps as app (app.clientID)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<div class="grid gap-0.5">
								<span class="text-sm font-medium">{app.name}</span>
								<span class="text-xs text-muted-foreground">
									{text.connectedOn.replace('{date}', new Date(app.grantedAt).toLocaleDateString())}
								</span>
							</div>
							<Button
								variant="ghost"
								size="sm"
								onclick={() => disconnect(app)}
								disabled={disconnectingID === app.clientID}
							>
								{text.disconnectApp}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
		</Card.Content>
	{/if}
</Card.Root>
