<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		forgetPersonalKey,
		issuePersonalKey,
		personalKeys,
		type PersonalKey
	} from '$lib/member/personal-keys';
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

	let keys = $state<PersonalKey[]>([]);
	let isLoading = $state(true);
	let keyName = $state('');
	let keyPermission = $state<PublicAPIPermission>(fullPublicAPIPermission);
	let isWorking = $state(false);
	let forgettingName = $state('');
	let issuedKey = $state('');

	const callExample = $derived(`${page.url.origin}/api/member/me`);

	const permissionLabels = $derived<Record<PublicAPIPermission, string>>({
		read: text.personalKeyReads,
		write: text.personalKeyWrites,
		delete: text.personalKeyDeletes
	});

	async function load() {
		try {
			keys = await personalKeys();
		} catch {
			toast.error(text.personalKeysLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function issue() {
		const name = keyName.trim();
		if (!name) {
			toast.error(text.personalKeyNeedsName);
			return;
		}
		isWorking = true;
		try {
			issuedKey = await issuePersonalKey(name, keyPermission);
			keyName = '';
			toast.success(text.personalKeyIssued);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalKeyFailed);
		} finally {
			isWorking = false;
		}
	}

	async function forget(name: string) {
		forgettingName = name;
		try {
			await forgetPersonalKey(name);
			toast.success(text.personalKeyRevoked);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalKeyFailed);
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
			{text.personalKeys}
		</Card.Title>
		<Card.Description>{text.personalKeysDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if issuedKey}
			<div class="grid gap-2 rounded-md border border-dashed p-4">
				<p class="text-sm text-muted-foreground">{text.personalKeyShownOnce}</p>
				<div class="flex items-center gap-2">
					<code class="min-w-0 flex-1 overflow-x-auto rounded bg-muted px-3 py-2 font-mono text-xs">
						{issuedKey}
					</code>
					<CopyButton text={issuedKey} />
				</div>
				<p class="text-sm text-muted-foreground">{text.personalKeyHowToUse}</p>
				<code class="overflow-x-auto rounded bg-muted px-3 py-2 font-mono text-xs">
					curl {callExample} --header "Authorization: Bearer {issuedKey}"
				</code>
			</div>
		{/if}

		<Field.Field>
			<Field.Label for="personal-key-name">{text.personalKeyName}</Field.Label>
			<Input
				id="personal-key-name"
				bind:value={keyName}
				placeholder={text.personalKeyNamePlaceholder}
				maxlength={64}
				onkeydown={(event) => {
					if (event.key === 'Enter') issue();
				}}
			/>
		</Field.Field>

		<Field.Field>
			<Field.Label for="personal-key-permission">{text.personalKeyPermission}</Field.Label>
			<div class="flex gap-2">
				<Select.Root type="single" bind:value={keyPermission} disabled={isWorking}>
					<Select.Trigger id="personal-key-permission" class="flex-1">
						{permissionLabels[keyPermission]}
					</Select.Trigger>
					<Select.Content>
						{#each publicAPIPermissions as permission (permission)}
							<Select.Item value={permission} label={permissionLabels[permission]}>
								{permissionLabels[permission]}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<Button onclick={issue} disabled={isWorking}>{text.issuePersonalKey}</Button>
			</div>
		</Field.Field>

		{#if !isLoading}
			{#if keys.length === 0}
				<p class="text-sm text-muted-foreground">{text.noPersonalKeys}</p>
			{:else}
				<ul class="grid gap-2">
					{#each keys as key (key.name)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<span class="min-w-0 truncate text-sm font-medium">{key.name}</span>
							<div class="flex items-center gap-2">
								<span class="text-sm text-muted-foreground">{permissionLabels[key.permission]}</span>
								<Button
									variant="ghost"
									size="sm"
									onclick={() => forget(key.name)}
									disabled={forgettingName === key.name}
								>
									{text.revokePersonalKey}
								</Button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</Card.Content>
</Card.Root>
