<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { onMount } from 'svelte';
	import { isCompanionVerified, isStalePairingStatus, normalizeManualPairingInput, parsePairingLink, stalePairingMessage, statusLabel, type CompanionStatus } from './lib/pairing';
	import { disconnectCompanion, ensureLaunchAtLogin, pairCompanion, readCompanionStatus, refreshRuntimeStatus, restartCompanionRuntime, setRuntimeState, startCompanionRuntime } from './lib/sidecar';
	import PairingPanel from './lib/components/PairingPanel.svelte';
	import RuntimeStatusPanel from './lib/components/RuntimeStatusPanel.svelte';
	import SettingsPanel from './lib/components/SettingsPanel.svelte';

	let status = $state<CompanionStatus>({ paired: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let isBusy = $state(false);

	onMount(() => {
		void bootstrap();
		void registerShellEvents();
	});

	async function bootstrap() {
		try {
			await ensureLaunchAtLogin();
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Launch at login setup failed';
		}
		await refreshStatus();
		if (isCompanionVerified(status)) {
			await ensureRuntime();
		}
	}

	async function refreshStatus() {
		try {
			status = await readCompanionStatus();
			if (isStalePairingStatus(status)) {
				message = stalePairingMessage;
			} else if (message === stalePairingMessage) {
				message = '';
			}
			await refreshRuntimeStatus();
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
			await refreshStatus();
			if (isCompanionVerified(status)) {
				await restartRuntime();
			}
			message = 'Connected. You can close this window.';
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Pairing failed';
		} finally {
			isBusy = false;
		}
	}

	async function disconnect() {
		if (!window.confirm('Disconnect this Companion from Intern Kim?')) return;
		isBusy = true;
		message = '';
		try {
			await disconnectCompanion();
			status = await readCompanionStatus();
			message = 'Disconnected.';
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Disconnect failed';
		} finally {
			isBusy = false;
		}
	}

	async function ensureRuntime() {
		try {
			await startCompanionRuntime();
		} catch (errorValue) {
			setRuntimeState({ isRunning: false, lastError: errorValue instanceof Error ? errorValue.message : 'Runtime failed to start' });
		}
	}

	async function restartRuntime() {
		try {
			await restartCompanionRuntime();
		} catch (errorValue) {
			setRuntimeState({ isRunning: false, lastError: errorValue instanceof Error ? errorValue.message : 'Runtime failed to restart' });
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
		<div class:online={isCompanionVerified(status)} class="indicator">{isCompanionVerified(status) ? 'online' : 'not paired'}</div>
	</section>

	<RuntimeStatusPanel {status} onStart={ensureRuntime} />


	<SettingsPanel {status} />


	<PairingPanel
		{status}
		{message}
		{isBusy}
		bind:deviceURL
		bind:pairingCode
		onPairManually={pairManually}
		onDisconnect={disconnect}
		onOpenAdmin={openAdmin}
		onCloseWindow={closeWindow}
	/>
</main>
