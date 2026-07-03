<script lang="ts">
	import { isCompanionVerified, isStalePairingStatus, type CompanionStatus } from '../pairing';

	let {
		status,
		message,
		isBusy,
		deviceURL = $bindable(''),
		pairingCode = $bindable(''),
		onPairManually,
		onDisconnect,
		onOpenAdmin,
		onCloseWindow
	}: {
		status: CompanionStatus;
		message: string;
		isBusy: boolean;
		deviceURL?: string;
		pairingCode?: string;
		onPairManually: () => void;
		onDisconnect: () => void;
		onOpenAdmin: () => void;
		onCloseWindow: () => void;
	} = $props();
</script>

{#if isCompanionVerified(status)}
	<section class="form-panel">
		<h2>Connection</h2>
		<div class="runtime-row">
			<span>device</span>
			<span>{status.deviceURL}</span>
		</div>
		<div class="runtime-row">
			<span>companion</span>
			<span>{status.companionID}</span>
		</div>
		<div class="actions">
			<button class="secondary" disabled={!status.deviceURL} onclick={onOpenAdmin}>Open Admin</button>
			<button class="secondary" disabled={isBusy} onclick={onDisconnect}>{isBusy ? 'Disconnecting...' : 'Disconnect'}</button>
			<button class="ghost" onclick={onCloseWindow}>Hide</button>
		</div>
		{#if message}
			<p class="message">{message}</p>
		{/if}
	</section>
{:else}
	<section class="form-panel">
		<h2>{isStalePairingStatus(status) ? 'Reconnect manually' : 'Connect manually'}</h2>
		<label>
			<span>Device URL</span>
			<input bind:value={deviceURL} placeholder="https://device.example.test" />
		</label>
		<label>
			<span>Pairing code</span>
			<input bind:value={pairingCode} placeholder="ABCD-1234" />
		</label>
		<div class="actions">
			<button disabled={isBusy} onclick={onPairManually}>{isBusy ? 'Connecting...' : 'Connect'}</button>
			<button class="secondary" disabled={!status.deviceURL} onclick={onOpenAdmin}>Open Admin</button>
			<button class="ghost" onclick={onCloseWindow}>Hide</button>
		</div>
		{#if message}
			<p class="message">{message}</p>
		{/if}
	</section>
{/if}

<section class="capability-panel">
	<h2>Advertised capabilities</h2>
	{#if status.capabilities?.length}
		<div class="capabilities">
			{#each status.capabilities as capability}
				<span>{capability.name}</span>
			{/each}
		</div>
	{:else}
		<p class="subtle">Capabilities appear after pairing and runtime startup.</p>
	{/if}
</section>
