<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Item from '$lib/components/ui/item';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { boxStepOf, isCompanyServed, shortBoxName } from './box-step';
	import type { BoxClaimIntent, ConnectedBox, EmptyBox, VerifiedBoxAnswer } from '$lib/company/box';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import BoxAdminPassword from './box-admin-password.svelte';
	import BoxPlacement from './box-placement.svelte';
	import BoxTakeOver from './box-take-over.svelte';
	import BoxWifiChange from './box-wifi-change.svelte';
	import ConnectedComputer from './connected-computer.svelte';
	import { connectBox, disconnectBox, fetchBoxes, giveModelKey, makeBoxServe, verifyBoxCode, type Boxes } from './host-setup-client';
	import { hostSetupText } from './text';

	let { isCompanyComputerOnline }: { isCompanyComputerOnline: boolean } = $props();
	const text = createPageText(hostSetupText);
	const refreshMilliseconds = 5000;
	let boxes = $state<Boxes>({
		boxes: [],
		sealingKey: null,
		hasModelKey: false,
		newestBackupAt: null,
		takeOver: { isAutomatic: false, lapseSeconds: 300 },
		isServedByConnectionFile: false,
		empty: []
	});
	let modelKey = $state('');
	let isChangingModelKey = $state(false);
	let connectingKey = $state('');
	let pairingCodes = $state<Record<string, string>>({});
	let verified = $state<VerifiedBoxAnswer | null>(null);
	let takingOver = $state<ConnectedBox | null>(null);
	let isSendingModelKey = $state(false);
	let errorMessage = $state('');
	let refreshTimer: ReturnType<typeof setInterval> | undefined;
	const step = $derived(boxStepOf(boxes));
	const hasBoxes = $derived(boxes.boxes.length > 0);
	const hasSeveralBoxes = $derived(boxes.boxes.length > 1);
	const isServed = $derived(isCompanyServed(boxes));

	onMount(() => {
		void refresh();
		refreshTimer = setInterval(() => {
			if (step !== 'connected' || boxes.boxes.some((box) => !box.holdsCompanyKey)) void refresh();
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

	function roleOf(box: ConnectedBox): string {
		if (!box.holdsCompanyKey) return text.boxAwaitingCompanyKey;
		return box.role === 'service' ? text.boxRoleService : text.boxRoleStandby;
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

	async function confirm(intent: BoxClaimIntent) {
		if (!verified) return;
		const confirmed = verified;
		await whileConnecting(confirmed.publicKey, async () => {
			boxes = { ...(await connectBox(confirmed.publicKey, confirmed.ticket, intent)), empty: [] };
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

	function askToDisconnect(box: ConnectedBox) {
		confirmDelete({
			title: text.disconnectTitle,
			description: text.disconnectDescription,
			confirm: { text: text.disconnect },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				boxes = { ...(await disconnectBox(box)), empty: boxes.empty };
			}
		});
	}

	function canServe(box: ConnectedBox): boolean {
		return box.role === 'standby' && box.holdsCompanyKey && step === 'connected' && !boxes.isServedByConnectionFile;
	}

	function backupMoment(isoTime: string): string {
		return new Date(isoTime).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	}

	async function serveFrom(box: ConnectedBox) {
		await whileConnecting(box.publicKey, async () => {
			boxes = { ...(await makeBoxServe(box)), empty: boxes.empty };
		});
		takingOver = null;
	}

	async function sendModelKey(event: SubmitEvent) {
		event.preventDefault();
		const [anyBox] = boxes.boxes;
		if (!anyBox || !boxes.sealingKey) return;
		if (!modelKey.trim()) {
			errorMessage = text.modelKeyMissing;
			return;
		}
		isSendingModelKey = true;
		errorMessage = '';
		try {
			boxes = { ...(await giveModelKey(anyBox.companyID, boxes.sealingKey, modelKey)), empty: boxes.empty };
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
	{#if isCompanyComputerOnline && (!hasBoxes || boxes.isServedByConnectionFile)}<ConnectedComputer />{/if}

	{#each boxes.boxes as box (box.publicKey)}
		<Item.Root variant="outline">
			<Item.Content>
				<Item.Title>{box.hostName ?? text.foundBox}</Item.Title>
				<Item.Description><span class="font-mono">{shortBoxName(box.publicKey)}</span> · {roleOf(box)}{box.site ? ` · ${box.site}` : ''}</Item.Description>
				{#if hasSeveralBoxes && box.lastSeenAt}
					<Item.Description>{text.lastSeen.replace('{when}', backupMoment(box.lastSeenAt))}</Item.Description>
				{/if}
			</Item.Content>
			<Item.Actions>
				{#if canServe(box)}
					<Button variant="outline" onclick={() => (takingOver = box)} disabled={connectingKey !== ''}>{text.makeServe}</Button>
				{/if}
				<Button variant="ghost" onclick={() => askToDisconnect(box)}>{text.disconnect}</Button>
			</Item.Actions>
		</Item.Root>
		{#if takingOver?.publicKey === box.publicKey}
			<Item.Root variant="outline">
				<Item.Content>
					<Item.Title>{text.makeServeTitle.replace('{box}', box.hostName ?? shortBoxName(box.publicKey))}</Item.Title>
					{#if boxes.newestBackupAt}
						<Item.Description>{text.makeServeFromBackup.replace('{when}', backupMoment(boxes.newestBackupAt))}</Item.Description>
						<Item.Description>{text.makeServeOld}</Item.Description>
					{:else}
						<Item.Description>{text.makeServeNeedsBackup}</Item.Description>
					{/if}
				</Item.Content>
				<Item.Actions>
					<Button variant="outline" onclick={() => (takingOver = null)} disabled={connectingKey !== ''}>{text.cancel}</Button>
					<Button onclick={() => serveFrom(box)} disabled={connectingKey !== '' || !boxes.newestBackupAt}>
						{connectingKey === box.publicKey ? text.makingServe : text.makeServe}
					</Button>
				</Item.Actions>
			</Item.Root>
		{/if}
		{#if step === 'connected'}
			{#if hasSeveralBoxes}<BoxPlacement {box} onPlaced={(placed) => (boxes = { ...placed, empty: boxes.empty })} />{/if}
			<BoxAdminPassword {box} />
			<BoxWifiChange {box} />
		{/if}
	{/each}

	{#if hasBoxes}
		<p role="status" class="text-sm">{step === 'connected' ? text.boxConnected : text.boxClaimed}</p>
		{#if hasSeveralBoxes}
			<p class="text-sm text-muted-foreground">
				{boxes.newestBackupAt ? text.newestBackup.replace('{when}', backupMoment(boxes.newestBackupAt)) : text.noBackupYet}
			</p>
			{#if step === 'connected'}
				<BoxTakeOver takeOver={boxes.takeOver} onKept={(kept) => (boxes = { ...kept, empty: boxes.empty })} />
			{/if}
		{/if}
		{#if step === 'connected' && !isChangingModelKey}
			<Button variant="outline" class="justify-self-start" onclick={() => (isChangingModelKey = true)}>{text.changeModelKey}</Button>
		{/if}
	{/if}

	{#if verified}
		<Item.Root variant="outline">
			<Item.Content>
				<Item.Title>{text.confirmBox.replace('{company}', verified.companyName)}</Item.Title>
				<Item.Description>{verified.hostName ?? text.foundBox} · <span class="font-mono">{shortBoxName(verified.publicKey)}</span></Item.Description>
				<Item.Description>{text.confirmNetwork.replace('{address}', verified.publicAddress)}</Item.Description>
				<Item.Description>{text.confirmMatch.replace('{fingerprint}', shortBoxName(verified.publicKey))}</Item.Description>
				{#if boxes.isServedByConnectionFile}
					<Item.Description>{text.confirmAddBesideComputer}</Item.Description>
				{:else if hasBoxes}
					<Item.Description>{text.confirmAddOrReplace}</Item.Description>
				{/if}
			</Item.Content>
			<Item.Actions>
				<Button variant="outline" onclick={() => (verified = null)} disabled={connectingKey !== ''}>{text.cancel}</Button>
				{#if isServed}
					<Button variant="outline" onclick={() => confirm('replace')} disabled={connectingKey !== ''}>{text.confirmReplace}</Button>
				{/if}
				<Button onclick={() => confirm('add')} disabled={connectingKey !== ''}>
					{connectingKey === verified.publicKey ? text.connecting : isServed ? text.confirmAdd : text.confirmConnect}
				</Button>
			</Item.Actions>
		</Item.Root>
	{:else if step === 'searching' && !isCompanyComputerOnline}
		<div role="status" class="grid gap-1">
			<p class="flex items-center gap-2 text-sm font-medium"><Spinner />{text.searching}</p>
			<p class="text-sm text-muted-foreground">{text.searchingHint}</p>
		</div>
	{:else if boxes.empty.length > 0 && (isServed || isCompanyComputerOnline)}
		<Collapsible.Root class="grid gap-2">
			<Collapsible.Trigger class="inline-flex min-h-11 items-center justify-self-start text-sm underline">
				{text.showWaitingBoxes.replace('{count}', String(boxes.empty.length))}
			</Collapsible.Trigger>
			<Collapsible.Content class="grid gap-2">
				{#if isServed}<p class="text-sm text-muted-foreground">{text.addBoxDescription}</p>{/if}
				{@render waitingBoxes()}
			</Collapsible.Content>
		</Collapsible.Root>
	{:else if boxes.empty.length > 0}
		<p class="text-sm font-medium">{text.waitingBoxes}</p>
		{@render waitingBoxes()}
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
