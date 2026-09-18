<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import MonitorIcon from '@lucide/svelte/icons/monitor';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import {
		disconnectMyCompanion,
		fetchMyCompanions,
		issueCompanionPairing,
		type CompanionPairing,
		type MyCompanion
	} from './my-computer-api';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);
	const pairingPollMilliseconds = 5_000;

	let companions = $state<MyCompanion[]>([]);
	let pairing = $state<CompanionPairing | null>(null);
	let isLoading = $state(true);
	let isConnecting = $state(false);
	let disconnectingID = $state('');

	const steps = $derived(
		pairing
			? [
					{ label: text.myComputerInstallStep, command: pairing.installCommand },
					{ label: text.myComputerPairStep, command: pairing.pairCommand },
					{ label: text.myComputerServiceStep, command: pairing.serviceCommand }
				]
			: []
	);

	async function load() {
		try {
			companions = await fetchMyCompanions();
		} catch {
			toast.error(text.myComputerLoadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function connect() {
		isConnecting = true;
		try {
			pairing = await issueCompanionPairing();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.myComputerConnectFailed);
		} finally {
			isConnecting = false;
		}
	}

	async function disconnect(companionID: string) {
		disconnectingID = companionID;
		try {
			await disconnectMyCompanion(companionID);
			toast.success(text.myComputerDisconnected);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.myComputerDisconnectFailed);
		} finally {
			disconnectingID = '';
		}
	}

	async function noticeWhenPaired(knownIDs: Set<string>) {
		const seen = await fetchMyCompanions().catch(() => null);
		if (!seen) return;
		companions = seen;
		if (!seen.some((companion) => !knownIDs.has(companion.companionID))) return;
		pairing = null;
		toast.success(text.myComputerConnected);
	}

	$effect(() => {
		if (!pairing) return;
		const knownIDs = new Set(companions.map((companion) => companion.companionID));
		const expiresAt = new Date(pairing.expiresAt).getTime();
		const timer = setInterval(() => {
			if (Date.now() > expiresAt) {
				pairing = null;
				return;
			}
			void noticeWhenPaired(knownIDs);
		}, pairingPollMilliseconds);
		return () => clearInterval(timer);
	});

	function lastSeenText(companion: MyCompanion): string {
		return text.myComputerLastSeen.replace('{date}', new Date(companion.lastSeenAt).toLocaleString());
	}

	function expiryText(offered: CompanionPairing): string {
		return text.myComputerCodeExpires.replace('{time}', new Date(offered.expiresAt).toLocaleTimeString());
	}

	onMount(load);
</script>

<Card.Root>
	<Card.Header>
		<Card.Title class="flex items-center gap-2">
			<MonitorIcon class="size-4 text-muted-foreground" />
			{text.myComputer}
		</Card.Title>
		<Card.Description>{text.myComputerDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if !isLoading}
			{#if companions.length === 0}
				<p class="text-sm text-muted-foreground">{text.myComputerNone}</p>
			{:else}
				<ul class="grid gap-2">
					{#each companions as companion (companion.companionID)}
						<li class="flex items-center justify-between gap-4 rounded-md border px-4 py-3">
							<div class="grid min-w-0 gap-1">
								<div class="flex items-center gap-2">
									<span class="truncate text-sm font-medium">{companion.displayName}</span>
									<Badge variant={companion.isOnline ? 'default' : 'outline'}>
										{companion.isOnline ? text.myComputerOnline : text.myComputerOffline}
									</Badge>
									{#if companion.canControlComputer}
										<Badge variant="secondary">{text.myComputerControlsComputer}</Badge>
									{/if}
								</div>
								{#if companion.lastSeenAt}
									<span class="text-xs text-muted-foreground">{lastSeenText(companion)}</span>
								{/if}
							</div>
							<Button
								variant="ghost"
								size="sm"
								onclick={() => disconnect(companion.companionID)}
								disabled={disconnectingID === companion.companionID}
							>
								{text.myComputerDisconnect}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}

		{#if pairing}
			<div class="grid gap-3 rounded-md border border-dashed p-4">
				<p class="text-sm text-muted-foreground">{text.myComputerSteps}</p>
				{#each steps as step (step.label)}
					<div class="grid gap-1.5">
						<span class="text-sm font-medium">{step.label}</span>
						<div class="flex items-center gap-2">
							<code class="min-w-0 flex-1 overflow-x-auto rounded bg-muted px-3 py-2 font-mono text-xs">{step.command}</code>
							<CopyButton text={step.command} />
						</div>
					</div>
				{/each}
				<p class="text-xs text-muted-foreground">{expiryText(pairing)}</p>
			</div>
		{:else}
			<Button class="w-fit" onclick={connect} disabled={isLoading || isConnecting}>
				{companions.length === 0 ? text.myComputerConnect : text.myComputerConnectAnother}
			</Button>
		{/if}
	</Card.Content>
</Card.Root>
