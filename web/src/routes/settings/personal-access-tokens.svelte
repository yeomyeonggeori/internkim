<script lang="ts">
	import SettingsListLoading from './settings-list-loading.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		forgetPersonalAccessToken,
		issuePersonalAccessToken,
		personalAccessTokens,
		type PersonalAccessToken
	} from '$lib/member/personal-access-tokens';
	import {
		fullPublicAPIPermission,
		publicAPIPermissions,
		type PublicAPIPermission
	} from '$lib/public-api-permission';
	import KeyIcon from '@lucide/svelte/icons/key-round';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);

	let keys = $state<PersonalAccessToken[]>([]);
	let isLoading = $state(true);
	let hasLoadError = $state(false);
	let keyName = $state('');
	let keyPermission = $state<PublicAPIPermission>(fullPublicAPIPermission);
	let isWorking = $state(false);
	let forgettingName = $state('');
	let issuedKey = $state('');

	const callExample = $derived(`${page.url.origin}/api/member/me`);

	const permissionLabels = $derived<Record<PublicAPIPermission, string>>({
		read: text.personalAccessTokenReads,
		write: text.personalAccessTokenWrites,
		delete: text.personalAccessTokenDeletes
	});

	function withDate(template: string, moment: string): string {
		return template.replace('{date}', new Date(moment).toLocaleDateString());
	}

	function lifetimeOf(key: PersonalAccessToken): string {
		if (Date.parse(key.expiresAt) <= Date.now()) return text.personalAccessTokenExpired;
		const usage = key.lastUsedAt ? withDate(text.lastUsedOn, key.lastUsedAt) : text.neverUsed;
		return `${withDate(text.personalAccessTokenExpiresOn, key.expiresAt)} · ${usage}`;
	}

	async function load() {
		hasLoadError = false;
		try {
			keys = await personalAccessTokens();
		} catch {
			hasLoadError = true;
			toast.error(text.personalAccessTokensLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function issue() {
		const name = keyName.trim();
		if (!name) {
			toast.error(text.personalAccessTokenNeedsName);
			return;
		}
		isWorking = true;
		try {
			issuedKey = await issuePersonalAccessToken(name, keyPermission);
			keyName = '';
			toast.success(text.personalAccessTokenIssued);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalAccessTokenFailed);
		} finally {
			isWorking = false;
		}
	}

	async function forget(name: string) {
		forgettingName = name;
		try {
			await forgetPersonalAccessToken(name);
			toast.success(text.personalAccessTokenRevoked);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalAccessTokenFailed);
		} finally {
			forgettingName = '';
		}
	}

	onMount(load);
</script>

<Card.Root>
	<Card.Header>
		<Card.Title class="flex items-center gap-2">
			<KeyIcon class="size-4 text-muted-foreground" />
			{text.personalAccessTokens}
		</Card.Title>
		<Card.Description>{text.personalAccessTokensDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if issuedKey}
			<div class="grid gap-2 rounded-md border border-dashed p-4">
				<p class="text-sm text-muted-foreground">{text.personalAccessTokenShownOnce}</p>
				<div class="flex items-center gap-2">
					<code class="min-w-0 flex-1 overflow-x-auto rounded bg-muted px-3 py-2 font-mono text-xs">
						{issuedKey}
					</code>
					<CopyButton text={issuedKey} />
				</div>
				<p class="text-sm text-muted-foreground">{text.personalAccessTokenHowToUse}</p>
				<code class="overflow-x-auto rounded bg-muted px-3 py-2 font-mono text-xs">
					curl {callExample} --header "Authorization: Bearer {issuedKey}"
				</code>
			</div>
		{/if}

		<Field.Field>
			<Field.Label for="personal-access-token-name">{text.personalAccessTokenName}</Field.Label>
			<Input
				id="personal-access-token-name"
				bind:value={keyName}
				placeholder={text.personalAccessTokenNamePlaceholder}
				maxlength={64}
				onkeydown={(event) => {
					if (event.key === 'Enter') issue();
				}}
			/>
		</Field.Field>

		<Field.Field>
			<Field.Label for="personal-access-token-permission">{text.personalAccessTokenPermission}</Field.Label>
			<div class="flex gap-2">
				<Select.Root type="single" bind:value={keyPermission} disabled={isWorking}>
					<Select.Trigger id="personal-access-token-permission" class="flex-1">
						{permissionLabels[keyPermission]}
					</Select.Trigger>
					<Select.Content><Select.Group>
						{#each publicAPIPermissions as permission (permission)}
							<Select.Item value={permission} label={permissionLabels[permission]}>
								{permissionLabels[permission]}
							</Select.Item>
						{/each}
					</Select.Group></Select.Content>
				</Select.Root>
				<Button onclick={issue} disabled={isWorking}>{text.issuePersonalAccessToken}</Button>
			</div>
		</Field.Field>

		{#if isLoading}
			<SettingsListLoading label={text.personalAccessTokens} />
		{:else}
			{#if hasLoadError}<p role="alert" class="text-sm text-destructive">{text.personalAccessTokensLoadFailed}</p>{/if}
			{#if keys.length === 0 && !hasLoadError}
				<p class="text-sm text-muted-foreground">{text.noPersonalAccessTokens}</p>
			{:else}
				<ul class="grid gap-2">
					{#each keys as key (key.name)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<div class="grid min-w-0 gap-0.5">
								<span class="truncate text-sm font-medium">{key.name}</span>
								<span class="text-xs text-muted-foreground">{lifetimeOf(key)}</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="text-sm text-muted-foreground">{permissionLabels[key.permission]}</span>
								<Button
									variant="ghost"
									size="sm"
									onclick={() => forget(key.name)}
									disabled={forgettingName === key.name}
								>
									{text.revokePersonalAccessToken}
								</Button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</Card.Content>
</Card.Root>
