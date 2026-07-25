<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import type { AdminPageText, BuzzInviteRecord } from './admin-types';

	type BuzzSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: BuzzSectionProps = $props();

	let loadedBuzzBaseURL = $state('');
	let deepLink = $state('');
	let inviteName = $state('');
	let inviteEmail = $state('');
	let createdInviteURL = $state('');
	let createdIdentityKey = $state('');
	let invites = $state<BuzzInviteRecord[]>([]);
	let unlinkedPubkeys = $state<string[]>([]);
	let manualLinkEmails = $state<Record<string, string>>({});
	let errorMessage = $state('');
	let isCreatingInvite = $state(false);

	const buzzBaseURL = $derived(adminBaseURL ? adminBaseURL.replace(/\/admin\/api$/, '') + '/buzz/api' : '');

	$effect(() => {
		if (!buzzBaseURL || loadedBuzzBaseURL === buzzBaseURL) return;
		loadedBuzzBaseURL = buzzBaseURL;
		loadBuzzState();
	});

	type BuzzConfigResponse = { relayURL?: string; deepLink?: string };
	type BuzzInvitesResponse = { invites?: BuzzInviteRecord[]; unlinkedPubkeys?: string[] };
	type BuzzInviteCreatedResponse = { inviteURL?: string; identityPrivateKey?: string; identityPubkey?: string };

	async function loadBuzzState() {
		errorMessage = '';
		try {
			const [configResponse, invitesResponse] = await Promise.all([
				fetchBuzzJSON<BuzzConfigResponse>('/config'),
				fetchBuzzJSON<BuzzInvitesResponse>('/invites')
			]);
			deepLink = configResponse.deepLink ?? '';
			invites = invitesResponse.invites ?? [];
			unlinkedPubkeys = invitesResponse.unlinkedPubkeys ?? [];
		} catch {
			errorMessage = text.buzz.loadError;
		}
	}

	async function fetchBuzzJSON<ResponseDocument>(path: string, body?: object): Promise<ResponseDocument> {
		const response = await fetch(buzzBaseURL + path, {
			method: body ? 'POST' : 'GET',
			credentials: 'include',
			headers: body ? { 'Content-Type': 'application/json' } : undefined,
			body: body ? JSON.stringify(body) : undefined
		});
		if (!response.ok) throw new Error(await response.text());
		return (await response.json()) as ResponseDocument;
	}

	async function createInvite() {
		if (!inviteName.trim() || !inviteEmail.trim()) return;
		isCreatingInvite = true;
		errorMessage = '';
		try {
			const created = await fetchBuzzJSON<BuzzInviteCreatedResponse>('/invites', { name: inviteName.trim(), email: inviteEmail.trim() });
			createdInviteURL = created.inviteURL ?? '';
			createdIdentityKey = created.identityPrivateKey ?? '';
			inviteName = '';
			inviteEmail = '';
			await loadBuzzState();
		} catch {
			errorMessage = text.buzz.createError;
		} finally {
			isCreatingInvite = false;
		}
	}

	async function linkPubkey(pubkey: string) {
		const email = manualLinkEmails[pubkey]?.trim();
		if (!email) return;
		errorMessage = '';
		try {
			await fetchBuzzJSON('/links', { pubkey, email });
			await loadBuzzState();
		} catch {
			errorMessage = text.buzz.linkError;
		}
	}

	function inviteStatus(invite: BuzzInviteRecord) {
		if (invite.claimedPubkey) return text.buzz.statusJoined;
		if (new Date(invite.expiresAt).getTime() < Date.now()) return text.buzz.statusExpired;
		return text.buzz.statusPending;
	}
</script>

<div class="grid gap-5">
	<div class="rounded-lg border p-4">
		<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
			<div>
				<h3 class="text-sm font-semibold">{text.buzz.title}</h3>
				<p class="text-muted-foreground mt-1 text-sm">{text.buzz.description}</p>
			</div>
			{#if deepLink}
				<div class="flex items-center gap-2">
					<Button href={deepLink} class="gap-2" size="sm">
						<ExternalLinkIcon class="size-4" />
						{text.buzz.open}
					</Button>
					<CopyButton text={deepLink} variant="outline" size="sm" />
				</div>
			{/if}
		</div>
		<div class="grid gap-3 md:grid-cols-[1fr_1fr_auto]">
			<Input bind:value={inviteName} placeholder={text.buzz.namePlaceholder} disabled={!isDeviceReachable} />
			<Input bind:value={inviteEmail} placeholder={text.buzz.emailPlaceholder} type="email" disabled={!isDeviceReachable} />
			<Button disabled={!isDeviceReachable || isCreatingInvite || !inviteName.trim() || !inviteEmail.trim()} onclick={createInvite}>
				{#if isCreatingInvite}
					<LoaderIcon class="size-4 animate-spin" />
				{/if}
				{text.buzz.createInvite}
			</Button>
		</div>
		{#if createdInviteURL}
			<div class="mt-3 flex flex-wrap items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
				<span class="min-w-0 flex-1 truncate text-sm">{createdInviteURL}</span>
				<CopyButton text={createdInviteURL} variant="outline" size="sm" />
			</div>
		{/if}
		{#if createdIdentityKey}
			<div class="mt-2 rounded-md border bg-muted/30 px-3 py-2">
				<div class="flex flex-wrap items-center gap-2">
					<span class="text-muted-foreground text-xs font-medium">{text.buzz.identityKeyLabel}</span>
					<code class="min-w-0 flex-1 truncate text-xs">{createdIdentityKey}</code>
					<CopyButton text={createdIdentityKey} variant="outline" size="sm" />
				</div>
				<p class="text-muted-foreground mt-1 text-xs">{text.buzz.identityKeyNotice}</p>
			</div>
		{/if}
	</div>

	{#if invites.length > 0}
		<div class="rounded-lg border p-4">
			<h3 class="mb-3 text-sm font-semibold">{text.buzz.invitesTitle}</h3>
			<div class="grid gap-2">
				{#each invites as invite}
					<div class="flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm">
						<span class="font-medium">{invite.name}</span>
						<span class="text-muted-foreground min-w-0 flex-1 truncate">{invite.email}</span>
						<Badge variant={invite.claimedPubkey ? 'default' : 'outline'}>{inviteStatus(invite)}</Badge>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if unlinkedPubkeys.length > 0}
		<div class="rounded-lg border p-4">
			<h3 class="mb-1 text-sm font-semibold">{text.buzz.unlinkedTitle}</h3>
			<p class="text-muted-foreground mb-3 text-sm">{text.buzz.unlinkedDescription}</p>
			<div class="grid gap-2">
				{#each unlinkedPubkeys as pubkey}
					<div class="flex flex-wrap items-center gap-2">
						<code class="text-muted-foreground min-w-0 flex-1 truncate text-xs">{pubkey}</code>
						<Input class="w-56" bind:value={manualLinkEmails[pubkey]} placeholder={text.buzz.emailPlaceholder} type="email" />
						<Button size="sm" variant="outline" onclick={() => linkPubkey(pubkey)}>{text.buzz.link}</Button>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if errorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
			{errorMessage}
		</p>
	{/if}
</div>
