<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import {
		forgetPasskey,
		isPasskeySupported,
		listPasskeys,
		passkeyFailureText,
		refusalOf,
		registerPasskey,
		type Passkey
	} from '$lib/supabase-passkey';
	import FingerprintIcon from '@lucide/svelte/icons/fingerprint';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const isSupported = isPasskeySupported();

	let passkeys = $state<Passkey[]>([]);
	let isLoading = $state(true);
	let removingID = $state('');
	let isRegistering = $state(false);

	function nameOf(passkey: Passkey): string {
		return passkey.name || text.unnamedPasskey;
	}

	function withDate(template: string, moment: string): string {
		return template.replace('{date}', new Date(moment).toLocaleDateString());
	}

	function usageOf(passkey: Passkey): string {
		if (!passkey.lastUsedAt) return text.neverUsed;
		return withDate(text.lastUsedOn, passkey.lastUsedAt);
	}

	async function load() {
		try {
			passkeys = await listPasskeys();
		} catch {
			toast.error(text.passkeysLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function add() {
		isRegistering = true;
		try {
			await registerPasskey();
			toast.success(text.passkeyAdded);
			await load();
		} catch (error) {
			const refusal = refusalOf(error);
			if (refusal === 'already-registered') toast.error(text.passkeyAlreadyRegistered);
			else if (refusal === 'failed') toast.error(passkeyFailureText(text.passkeyFailed, error));
		} finally {
			isRegistering = false;
		}
	}

	async function remove(passkey: Passkey) {
		removingID = passkey.id;
		try {
			await forgetPasskey(passkey.id);
			toast.success(text.passkeyRemoved);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.passkeyFailed);
		} finally {
			removingID = '';
		}
	}

	onMount(load);
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.passkeys}</Card.Title>
		<Card.Description>{text.signInDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if !isSupported}
			<p class="text-sm text-muted-foreground">{text.passkeysUnsupported}</p>
		{:else if !isLoading}
			{#if passkeys.length === 0}
				<p class="text-sm text-muted-foreground">{text.noPasskeys}</p>
			{:else}
				<ul class="grid gap-2">
					{#each passkeys as passkey (passkey.id)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<div class="grid gap-0.5">
								<span class="text-sm font-medium">{nameOf(passkey)}</span>
								<span class="text-xs text-muted-foreground">
									{withDate(text.addedOn, passkey.createdAt)} · {usageOf(passkey)}
								</span>
							</div>
							<Button
								variant="ghost"
								size="sm"
								onclick={() => remove(passkey)}
								disabled={removingID === passkey.id}
							>
								{text.removePasskey}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
			<Button class="w-full gap-2" onclick={add} disabled={isRegistering}>
				<FingerprintIcon class="size-4" />
				{text.addPasskey}
			</Button>
		{/if}
	</Card.Content>
</Card.Root>
