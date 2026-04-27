<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { onMount } from 'svelte';
	import { normalizeManualPairingInput, parsePairingLink, statusLabel, type CompanionStatus } from './lib/pairing';
	import { confirmResponse, inputResponse, normalizePromptRequest, promptTitle, type PromptRequest, type PromptResult } from './lib/prompts';
	import { pairCompanion, readCompanionStatus, readRuntimeStatus, startCompanionRuntime, type RuntimeStatus } from './lib/sidecar';

	let status = $state<CompanionStatus>({ paired: false });
	let runtime = $state<RuntimeStatus>({ isRunning: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let promptInput = $state('');
	let pendingPrompt = $state<PromptRequest | undefined>();
	let promptResult = $state<PromptResult>({ status: 'idle' });
	let isBusy = $state(false);

	onMount(() => {
		void bootstrap();
		void registerShellEvents();
		const intervalID = window.setInterval(() => {
			runtime = readRuntimeStatus();
		}, 1000);
		return () => window.clearInterval(intervalID);
	});

	async function bootstrap() {
		await refreshStatus();
		if (status.paired) {
			await ensureRuntime();
		}
	}

	async function refreshStatus() {
		try {
			status = await readCompanionStatus();
			runtime = readRuntimeStatus();
		} catch {
			status = { paired: false };
			runtime = readRuntimeStatus();
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
					pendingPrompt = normalizePromptRequest(event.payload);
					promptInput = '';
					promptResult = { status: 'pending' };
					await invoke('show_main_window');
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
			await ensureRuntime();
			await refreshStatus();
			message = 'Connected. You can close this window.';
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Pairing failed';
		} finally {
			isBusy = false;
		}
	}

	async function ensureRuntime() {
		try {
			await startCompanionRuntime();
			runtime = readRuntimeStatus();
		} catch (errorValue) {
			runtime = { isRunning: false, lastError: errorValue instanceof Error ? errorValue.message : 'Runtime failed to start' };
		}
	}

	async function completePrompt(response: unknown) {
		if (!pendingPrompt) return;
		const requestID = pendingPrompt.requestID;
		try {
			await invoke('complete_prompt_request', { requestId: requestID, response });
			pendingPrompt = undefined;
			promptInput = '';
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
		<div class:online={status.paired} class="indicator">{status.paired ? 'online' : 'not paired'}</div>
	</section>

	<section class="runtime-panel">
		<h2>Runtime</h2>
		<div class="runtime-row">
			<span>{runtime.isRunning ? `running${runtime.processID ? ` #${runtime.processID}` : ''}` : 'stopped'}</span>
			<button class="secondary" disabled={!status.paired || runtime.isRunning} onclick={ensureRuntime}>Start</button>
		</div>
		{#if runtime.lastError}
			<p class="message error">{runtime.lastError}</p>
		{/if}
	</section>

	<section class="prompt-panel">
		<h2>Pending user request</h2>
		{#if pendingPrompt}
			<div class="prompt-card">
				<p class="eyebrow">{promptTitle(pendingPrompt)}</p>
				<p class="prompt-message">{pendingPrompt.message}</p>
				{#if pendingPrompt.kind === 'confirm'}
					<div class="actions">
						<button onclick={() => completePrompt(confirmResponse(true))}>Approve</button>
						<button class="secondary" onclick={() => completePrompt(confirmResponse(false))}>Deny</button>
					</div>
				{:else}
					<label>
						<span>Response</span>
						<input bind:value={promptInput} placeholder="Type your answer" />
					</label>
					<div class="actions">
						<button onclick={() => completePrompt(inputResponse(promptInput))}>Submit</button>
						<button class="secondary" onclick={() => completePrompt(inputResponse(''))}>Cancel</button>
					</div>
				{/if}
			</div>
		{:else if promptResult.status === 'completed' || promptResult.status === 'failed'}
			<p class:failed={promptResult.status === 'failed'} class="message">{promptResult.message}</p>
		{:else}
			<p class="subtle">Requests that need your confirmation or input will appear here.</p>
		{/if}
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
