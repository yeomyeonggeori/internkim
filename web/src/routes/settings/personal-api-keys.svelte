<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { forgetPersonalKey, issuePersonalKey, personalKeyNames } from '$lib/member/personal-keys';
	import KeyIcon from '@lucide/svelte/icons/key-round';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);

	let names = $state<string[]>([]);
	let isLoading = $state(true);
	let keyName = $state('');
	let isWorking = $state(false);
	let forgettingName = $state('');
	let issuedKey = $state('');

	const callExample = $derived(`${page.url.origin}/api/member/me`);

	async function load() {
		try {
			names = await personalKeyNames();
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
			issuedKey = await issuePersonalKey(name);
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
				<Button onclick={issue} disabled={isWorking}>{text.issuePersonalKey}</Button>
			</div>
		</Field.Field>

		{#if !isLoading}
			{#if names.length === 0}
				<p class="text-sm text-muted-foreground">{text.noPersonalKeys}</p>
			{:else}
				<ul class="grid gap-2">
					{#each names as name (name)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<span class="text-sm font-medium">{name}</span>
							<Button
								variant="ghost"
								size="sm"
								onclick={() => forget(name)}
								disabled={forgettingName === name}
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
