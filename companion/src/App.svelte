<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { onMount } from 'svelte';
	import { isCompanionVerified, isStalePairingStatus, normalizeManualPairingInput, parsePairingLink, stalePairingMessage, statusLabel, type CompanionStatus } from './lib/pairing';
	import { approvalResponse, confirmResponse, inputResponse, normalizePromptRequest, type PromptRequest, type PromptResult } from './lib/prompts';
	import { disconnectCompanion, ensureLaunchAtLogin, pairCompanion, readCompanionStatus, refreshRuntimeStatus, restartCompanionRuntime, setRuntimeState, startCompanionRuntime } from './lib/sidecar';
	import GrantsPanel from './lib/components/GrantsPanel.svelte';
	import MountsPanel from './lib/components/MountsPanel.svelte';
	import PairingPanel from './lib/components/PairingPanel.svelte';
	import RuntimeStatusPanel from './lib/components/RuntimeStatusPanel.svelte';
	import SettingsPanel from './lib/components/SettingsPanel.svelte';

	let status = $state<CompanionStatus>({ paired: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let promptResult = $state<PromptResult>({ status: 'idle' });
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
			await listen<unknown>('prompt-request', async (event) => {
				try {
					const prompt = normalizePromptRequest(event.payload);
					await invoke('show_main_window');
					await answerPromptWithAlert(prompt);
				} catch (errorValue) {
					message = errorValue instanceof Error ? errorValue.message : 'Prompt request failed';
				}
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

	async function answerPromptWithAlert(prompt: PromptRequest) {
		promptResult = { status: 'pending' };
		if (prompt.kind === 'confirm') {
			await completePromptRequest(prompt.requestID, confirmResponse(window.confirm(prompt.message)));
			return;
		}
		if (prompt.kind === 'input') {
			await completePromptRequest(prompt.requestID, inputResponse(window.prompt(prompt.message) ?? ''));
			return;
		}
		const allowed = window.confirm(prompt.message);
		if (allowed) {
			const rememberSession = window.confirm('이번 세션 동안 같은 권한을 다시 묻지 않을까요?');
			await completePromptRequest(prompt.requestID, approvalResponse(true, '', rememberSession));
			return;
		}
		const denialReason = window.prompt('거부 이유나 대안이 있으면 입력하세요.', '') ?? '';
		await completePromptRequest(prompt.requestID, approvalResponse(false, denialReason));
	}

	async function completePromptRequest(requestID: string, response: unknown) {
		try {
			await invoke('complete_prompt_request', { requestId: requestID, response });
			promptResult = { status: 'completed', message: 'Response sent.' };
		} catch (errorValue) {
			promptResult = { status: 'failed', message: errorValue instanceof Error ? errorValue.message : 'Response failed' };
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

	<MountsPanel onMessage={(text) => (message = text)} />

	<SettingsPanel {status} />

	<GrantsPanel onMessage={(text) => (message = text)} />

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
