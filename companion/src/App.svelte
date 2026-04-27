<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { normalizeManualPairingInput, parsePairingLink, statusLabel, type CompanionStatus } from './lib/pairing';
	import { pairCompanion, readCompanionStatus, startCompanionRuntime } from './lib/sidecar';

	let status = $state<CompanionStatus>({ paired: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let isBusy = $state(false);

	refreshStatus();
	registerShellEvents();

	async function refreshStatus() {
		try {
			status = await readCompanionStatus();
		} catch {
			status = { paired: false };
		}
	}

	async function registerShellEvents() {
		try {
			const initialLinks = await getCurrent();
			if (initialLinks?.[0]) await pairFromLink(initialLinks[0]);
			await onOpenUrl(async (urls) => {
				if (urls[0]) await pairFromLink(urls[0]);
			});
			await listen('open-admin-request', () => {
				void openAdmin();
			});
		} catch {
			message = 'Companion shell events are unavailable.';
		}
	}

	async function pairFromLink(pairingLink: string) {
		await invoke('show_main_window');
		try {
			const payload = parsePairingLink(pairingLink);
			deviceURL = payload.deviceURL;
			pairingCode = payload.code;
			await runPairing(payload);
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Pairing link failed';
		}
	}

	async function pairManually() {
		try {
			await runPairing(normalizeManualPairingInput(deviceURL, pairingCode));
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Pairing failed';
		}
	}

	async function runPairing(payload: { deviceURL: string; code: string }) {
		isBusy = true;
		message = '';
		try {
			await pairCompanion(payload);
			await startCompanionRuntime();
			await refreshStatus();
			message = 'Connected. You can close this window.';
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Pairing failed';
		} finally {
			isBusy = false;
		}
	}

	async function openAdmin() {
		if (!status.deviceURL) return;
		await invoke('open_admin_url', { deviceUrl: status.deviceURL });
	}

	async function closeWindow() {
		await getCurrentWindow().hide();
	}
</script>

<main class="shell">
	<section class="status-panel">
		<div>
			<p class="eyebrow">Intern Kim Companion</p>
			<h1>{statusLabel(status)}</h1>
			<p class="subtle">
				Companion handles user-local browser, file, confirmation, and future local model work without exposing local state to Intern Kim.
			</p>
		</div>
		<div class:online={status.paired} class="indicator">{status.paired ? 'online' : 'not paired'}</div>
	</section>

	<section class="form-panel">
		<h2>Connect manually</h2>
		<label>
			<span>Device URL</span>
			<input bind:value={deviceURL} placeholder="https://device.intern.kim" />
		</label>
		<label>
			<span>Pairing code</span>
			<input bind:value={pairingCode} placeholder="ABCD-1234" />
		</label>
		<div class="actions">
			<button disabled={isBusy} onclick={pairManually}>{isBusy ? 'Connecting...' : 'Connect'}</button>
			<button class="secondary" disabled={!status.deviceURL} onclick={openAdmin}>Open Admin</button>
			<button class="ghost" onclick={closeWindow}>Hide</button>
		</div>
		{#if message}
			<p class="message">{message}</p>
		{/if}
	</section>

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
</main>
