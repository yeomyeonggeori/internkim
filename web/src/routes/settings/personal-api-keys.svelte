<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		issuePersonalKey,
		listPersonalKeys,
		revokePersonalKey,
		type PersonalKey
	} from '$lib/member/personal-keys';
	import KeyIcon from '@lucide/svelte/icons/key-round';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);

	let keys = $state<PersonalKey[]>([]);
	let isLoading = $state(true);
	let keyName = $state('');
	let isIssuing = $state(false);
	let revokingID = $state('');
	let issuedKey = $state('');

	function withDate(template: string, moment: string): string {
		return template.replace('{date}', new Date(moment).toLocaleDateString());
	}

	function usageOf(key: PersonalKey): string {
		if (!key.lastSeenAt) return text.personalKeyNeverSeen;
		return withDate(text.personalKeyLastSeen, key.lastSeenAt);
	}

	async function load() {
		try {
			keys = await listPersonalKeys();
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
		isIssuing = true;
		try {
			const issued = await issuePersonalKey(name);
			issuedKey = issued.apiKey;
			keyName = '';
			toast.success(text.personalKeyIssued);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalKeyFailed);
		} finally {
			isIssuing = false;
		}
	}

	async function revoke(key: PersonalKey) {
		revokingID = key.keyID;
		try {
			await revokePersonalKey(key.keyID);
			toast.success(text.personalKeyRevoked);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.personalKeyFailed);
		} finally {
			revokingID = '';
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
			</div>
		{/if}

		<Field.Field>
			<Field.Label for="personal-key-name">{text.personalKeyName}</Field.Label>
			<div class="flex gap-2">
				<Input
					id="personal-key-name"
					bind:value={keyName}
					placeholder={text.personalKeyNamePlaceholder}
					maxlength={64}
					onkeydown={(event) => {
						if (event.key === 'Enter') issue();
					}}
				/>
				<Button onclick={issue} disabled={isIssuing}>{text.issuePersonalKey}</Button>
			</div>
		</Field.Field>

		{#if !isLoading}
			{#if keys.length === 0}
				<p class="text-sm text-muted-foreground">{text.noPersonalKeys}</p>
			{:else}
				<ul class="grid gap-2">
					{#each keys as key (key.keyID)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<div class="grid gap-0.5">
								<span class="text-sm font-medium">{key.name}</span>
								<span class="text-xs text-muted-foreground">
									{withDate(text.addedOn, key.createdAt)} · {usageOf(key)}
								</span>
							</div>
							<Button
								variant="ghost"
								size="sm"
								onclick={() => revoke(key)}
								disabled={revokingID === key.keyID}
							>
								{text.revokePersonalKey}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</Card.Content>
</Card.Root>
