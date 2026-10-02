<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { maximumSSIDBytes, type ConnectedBox, type NearbyNetwork } from '$lib/company/box';
	import { changeBoxWifiNetwork, fetchWifiChangeStatus } from './box-wifi-client';
	import { hostSetupText } from './text';

	let { box }: { box: ConnectedBox } = $props();

	const text = createPageText(hostSetupText);
	const pollMilliseconds = 5000;

	let isOffered = $state(false);
	let isFormOpen = $state(false);
	const typedSSIDChoice = '';

	let ssid = $state('');
	let selectedSSID = $state(typedSSIDChoice);
	let nearbyNetworks = $state<NearbyNetwork[]>([]);
	let password = $state('');
	let isSending = $state(false);
	let trackedRequestID = $state<string | null>(null);
	let outcome = $state<'joined' | 'failed' | null>(null);
	let errorMessage = $state('');
	let pollTimer: ReturnType<typeof setInterval> | undefined;

	const sortedNetworks = $derived([...nearbyNetworks].sort((first, second) => second.signalPercent - first.signalPercent));
	const isTyping = $derived(sortedNetworks.length === 0 || selectedSSID === typedSSIDChoice);

	onMount(() => void loadStatus());
	onDestroy(() => clearInterval(pollTimer));

	async function loadStatus() {
		try {
			const status = await fetchWifiChangeStatus();
			nearbyNetworks = status.nearbyNetworks;
			isOffered = status.scannedAt !== null || status.pendingRequestID !== null || status.outcome !== null;
			if (status.pendingRequestID) {
				trackedRequestID = status.pendingRequestID;
				startPolling();
			} else if (status.outcome) {
				trackedRequestID = status.outcome.requestID;
				outcome = status.outcome.result;
			}
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}

	function labelOf(network: NearbyNetwork): string {
		const marks = [network.isConnected ? text.wifiConnectedMark : '', network.isSecured ? text.wifiSecuredMark : ''];
		return [network.ssid, ...marks.filter((mark) => mark !== '')].join(' ');
	}

	function preferredSSIDOf(networks: NearbyNetwork[]): string {
		const connected = networks.find((network) => network.isConnected);
		const strongest = [...networks].sort((first, second) => second.signalPercent - first.signalPercent)[0];
		return connected?.ssid ?? strongest?.ssid ?? typedSSIDChoice;
	}

	async function openForm() {
		isFormOpen = true;
		errorMessage = '';
		try {
			nearbyNetworks = (await fetchWifiChangeStatus()).nearbyNetworks;
			selectedSSID = preferredSSIDOf(nearbyNetworks);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}

	function closeForm() {
		isFormOpen = false;
		ssid = '';
		selectedSSID = typedSSIDChoice;
		password = '';
		errorMessage = '';
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const chosenSSID = isTyping ? ssid.trim() : selectedSSID;
		if (!chosenSSID) {
			errorMessage = text.wifiFieldsMissing;
			return;
		}
		if (new TextEncoder().encode(chosenSSID).length > maximumSSIDBytes) {
			errorMessage = text.wifiSSIDTooLong;
			return;
		}
		isSending = true;
		errorMessage = '';
		try {
			const sent = await changeBoxWifiNetwork(box, { ssid: chosenSSID, password });
			trackedRequestID = sent.requestID;
			outcome = null;
			isFormOpen = false;
			ssid = '';
			selectedSSID = typedSSIDChoice;
			password = '';
			startPolling();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSending = false;
		}
	}

	function startPolling() {
		clearInterval(pollTimer);
		pollTimer = setInterval(() => void poll(), pollMilliseconds);
	}

	async function poll() {
		if (!trackedRequestID) return;
		try {
			const status = await fetchWifiChangeStatus();
			if (status.outcome && status.outcome.requestID === trackedRequestID) {
				outcome = status.outcome.result;
				clearInterval(pollTimer);
			} else if (!status.pendingRequestID) {
				trackedRequestID = null;
				clearInterval(pollTimer);
			}
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}
</script>

{#if isOffered}
	<div class="grid gap-2">
		{#if !isFormOpen}
			<Button variant="outline" onclick={openForm}>{text.wifiChange}</Button>
		{/if}

		{#if isFormOpen}
			<form class="grid gap-2" onsubmit={submit}>
				{#if sortedNetworks.length > 0}
					<Field.Field>
						<Field.Label for="box-wifi-nearby">{text.wifiNearbyChoose}</Field.Label>
						<select
							id="box-wifi-nearby"
							bind:value={selectedSSID}
							class="dark:bg-input/30 border-input focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full rounded-md border bg-transparent px-2.5 text-base shadow-xs outline-none focus-visible:ring-3 md:text-sm"
						>
							{#each sortedNetworks as network (network.ssid)}
								<option value={network.ssid}>{labelOf(network)}</option>
							{/each}
							<option value={typedSSIDChoice}>{text.wifiTypeManually}</option>
						</select>
					</Field.Field>
				{:else}
					<p class="text-muted-foreground text-sm">{text.wifiNoNearbyReported}</p>
				{/if}
				{#if isTyping}
					<Field.Field>
						<Field.Label for="box-wifi-ssid">{text.wifiSSID}</Field.Label>
						<Input id="box-wifi-ssid" bind:value={ssid} autocomplete="off" />
					</Field.Field>
				{/if}
				<Field.Field>
					<Field.Label for="box-wifi-password">{text.wifiPassword}</Field.Label>
					<Input id="box-wifi-password" type="password" bind:value={password} autocomplete="off" />
				</Field.Field>
				<div class="flex gap-2">
					<Button type="submit" disabled={isSending}>{isSending ? text.wifiApplying : text.wifiApply}</Button>
					<Button type="button" variant="ghost" onclick={closeForm} disabled={isSending}>{text.cancel}</Button>
				</div>
			</form>
		{/if}

		{#if trackedRequestID}
			<p role="status" class="text-sm">
				{#if outcome === 'joined'}
					{text.wifiJoined}
				{:else if outcome === 'failed'}
					{text.wifiFailed}
				{:else}
					{text.wifiApplying}
				{/if}
			</p>
		{/if}

		{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
	</div>
{/if}
