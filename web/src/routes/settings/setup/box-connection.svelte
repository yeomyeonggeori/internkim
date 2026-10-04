<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { boxStepOf, shortBoxName } from './box-step';
	import type { EmptyBox, VerifiedBoxAnswer } from '$lib/company/box';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import BoxAdminPassword from './box-admin-password.svelte';
	import BoxWifiChange from './box-wifi-change.svelte';
	import ConnectedComputer from './connected-computer.svelte';
	import { connectBox, disconnectBox, fetchBoxes, giveBoxModelKey, verifyBoxCode, type Boxes } from './host-setup-client';
	import { hostSetupText } from './text';

	let { isCompanyComputerOnline }: { isCompanyComputerOnline: boolean } = $props();
	const text = createPageText(hostSetupText);
	const refreshMilliseconds = 5000;
	let boxes = $state<Boxes>({ connected: null, empty: [] });
	let modelKey = $state('');
	let isChangingModelKey = $state(false);
	let connectingKey = $state('');
	let pairingCodes = $state<Record<string, string>>({});
	let verified = $state<VerifiedBoxAnswer | null>(null);
	let isSendingModelKey = $state(false);
	let errorMessage = $state('');
	let refreshTimer: ReturnType<typeof setInterval> | undefined;
	const step = $derived(boxStepOf(boxes));

	onMount(() => {
		void refresh();
		refreshTimer = setInterval(() => {
			if (step !== 'connected') void refresh();
		}, refreshMilliseconds);
	});

	onDestroy(() => clearInterval(refreshTimer));

	async function refresh() {
		try {
			boxes = await fetchBoxes();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}

	function whereTheCodeIs(box: EmptyBox): string {
		const [address] = box.pairingPageAddresses;
		return address ? text.pairingCodeAt.replace('{address}', address) : text.pairingCodeOnTheBox;
	}

	async function verify(event: SubmitEvent, publicKey: string) {
		event.preventDefault();
		const pairingCode = (pairingCodes[publicKey] ?? '').trim();
		if (!pairingCode) {
			errorMessage = text.pairingCodeMissing;
			return;
		}
		await whileConnecting(publicKey, async () => {
			verified = await verifyBoxCode(publicKey, pairingCode);
		});
	}

	async function confirm() {
		if (!verified) return;
		const confirmed = verified;
		await whileConnecting(confirmed.publicKey, async () => {
			boxes = { connected: await connectBox(confirmed.publicKey, confirmed.ticket), empty: [] };
		});
		verified = null;
	}

	async function whileConnecting(publicKey: string, work: () => Promise<void>) {
		connectingKey = publicKey;
		errorMessage = '';
		try {
			await work();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			connectingKey = '';
		}
	}

	function askToDisconnect() {
		confirmDelete({
			title: text.disconnectTitle,
			description: text.disconnectDescription,
			confirm: { text: text.disconnect },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await disconnectBox();
				boxes = { connected: null, empty: [] };
			}
		});
	}

	async function sendModelKey(event: SubmitEvent) {
		event.preventDefault();
		if (!boxes.connected) return;
		if (!modelKey.trim()) {
			errorMessage = text.modelKeyMissing;
			return;
		}
		isSendingModelKey = true;
		errorMessage = '';
		try {
			boxes = { ...boxes, connected: await giveBoxModelKey(boxes.connected, modelKey) };
			modelKey = '';
			isChangingModelKey = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSendingModelKey = false;
		}
	}
</script>

{#snippet waitingBoxes()}
	<Item.Group class="gap-2">
		{#each boxes.empty as box (box.publicKey)}
			<Item.Root variant="outline">
				<Item.Content>
					<Item.Title>{box.hostName ?? text.foundBox}</Item.Title>
					<Item.Description class="font-mono">{shortBoxName(box.publicKey)}</Item.Description>
					<Item.Description>{whereTheCodeIs(box)}</Item.Description>
				</Item.Content>
				<Item.Actions>
					<form class="flex flex-wrap items-center gap-2" onsubmit={(event) => verify(event, box.publicKey)}>
						<Input
							class="w-36 font-mono uppercase"
							aria-label={text.pairingCode}
							placeholder="ABCD-EFGH"
							autocomplete="off"
							maxlength={32}
							bind:value={pairingCodes[box.publicKey]}
						/>
						<Button type="submit" disabled={connectingKey !== ''}>
							{connectingKey === box.publicKey ? text.connecting : text.connect}
						</Button>
					</form>
				</Item.Actions>
			</Item.Root>
		{/each}
	</Item.Group>
{/snippet}

<div class="grid min-w-0 gap-4">
	{#if isCompanyComputerOnline && !boxes.connected}<ConnectedComputer />{/if}
	{#if step === 'searching' && !isCompanyComputerOnline}
		<div role="status" class="grid gap-1">
			<p class="flex items-center gap-2 text-sm font-medium"><Spinner />{text.searching}</p>
			<p class="text-sm text-muted-foreground">{text.searchingHint}</p>
		</div>
	{:else if step === 'choosing' && verified}
		<Item.Root variant="outline">
			<Item.Content>
				<Item.Title>{text.confirmBox.replace('{company}', verified.companyName)}</Item.Title>
				<Item.Description>{verified.hostName ?? text.foundBox} · <span class="font-mono">{shortBoxName(verified.publicKey)}</span></Item.Description>
				<Item.Description>{text.confirmNetwork.replace('{address}', verified.publicAddress)}</Item.Description>
				<Item.Description>{text.confirmMatch.replace('{fingerprint}', shortBoxName(verified.publicKey))}</Item.Description>
			</Item.Content>
			<Item.Actions>
				<Button variant="outline" onclick={() => (verified = null)} disabled={connectingKey !== ''}>{text.cancel}</Button>
				<Button onclick={confirm} disabled={connectingKey !== ''}>
					{connectingKey === verified.publicKey ? text.connecting : text.confirmConnect}
				</Button>
			</Item.Actions>
		</Item.Root>
	{:else if step === 'choosing' && isCompanyComputerOnline}
		<Collapsible.Root class="grid gap-2">
			<Collapsible.Trigger class="inline-flex min-h-11 items-center justify-self-start text-sm underline">
				{text.showWaitingBoxes.replace('{count}', String(boxes.empty.length))}
			</Collapsible.Trigger>
			<Collapsible.Content>{@render waitingBoxes()}</Collapsible.Content>
		</Collapsible.Root>
	{:else if step === 'choosing'}
		<p class="text-sm font-medium">{text.waitingBoxes}</p>
		{@render waitingBoxes()}
	{:else if boxes.connected}
		<Item.Root variant="outline">
			<Item.Content>
				<Item.Title>{text.foundBox}</Item.Title>
				<Item.Description class="font-mono">{shortBoxName(boxes.connected.publicKey)}</Item.Description>
			</Item.Content>
			<Item.Actions>
				{#if step === 'connected' && !isChangingModelKey}
					<Button variant="outline" onclick={() => (isChangingModelKey = true)}>{text.changeModelKey}</Button>
				{/if}
				<Button variant="ghost" onclick={askToDisconnect}>{text.disconnect}</Button>
			</Item.Actions>
		</Item.Root>
		<p role="status" class="text-sm">{step === 'connected' ? text.boxConnected : text.boxClaimed}</p>
		{#if step === 'connected'}
			<BoxAdminPassword box={boxes.connected} />
			<BoxWifiChange box={boxes.connected} />
		{/if}
	{/if}

	{#if step === 'givingModelKey' || isChangingModelKey}
		<form onsubmit={sendModelKey}>
			<Field.Field>
				<Field.Label for="box-model-key">{text.modelKey}</Field.Label>
				<div class="flex flex-wrap items-center gap-2">
					<Input id="box-model-key" class="w-full min-w-0 sm:min-w-64 sm:flex-1" bind:value={modelKey} type="password" autocomplete="off" />
					<Button type="submit" disabled={isSendingModelKey}>
						{isSendingModelKey ? text.sendingModelKey : text.sendModelKey}
					</Button>
				</div>
				<Field.Description>
					{text.modelKeyDescription}
					<a class="underline" href="https://openrouter.ai/keys" target="_blank" rel="noreferrer">{text.modelKeyLink}</a>
				</Field.Description>
			</Field.Field>
		</form>
	{/if}

	{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
</div>
