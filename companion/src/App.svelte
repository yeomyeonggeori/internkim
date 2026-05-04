<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { onMount } from 'svelte';
	import { normalizeManualPairingInput, parsePairingLink, statusLabel, type CompanionStatus } from './lib/pairing';
	import { approvalResponse, confirmResponse, inputResponse, normalizePromptRequest, promptTitle, type PromptRequest, type PromptResult } from './lib/prompts';
	import { addMountedFolder, pairCompanion, pauseMountedFolder, readActiveGrants, readCompanionStatus, readMountedFolders, readRuntimeStatus, refreshRuntimeStatus, restartCompanionRuntime, resumeMountedFolder, revokeGrant, revokeMountedFolder, startCompanionRuntime, type ActiveGrant, type MountedFolder, type RuntimeStatus } from './lib/sidecar';
	import { defaultSettings, fetchBackendModels, loadCompanionSettings, saveCompanionSettings, type CompanionSettings } from './lib/settings';

	let status = $state<CompanionStatus>({ paired: false });
	let runtime = $state<RuntimeStatus>({ isRunning: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let promptInput = $state('');
	let denialReason = $state('');
	let pendingPrompt = $state<PromptRequest | undefined>();
	let promptResult = $state<PromptResult>({ status: 'idle' });
	let activeGrants = $state<ActiveGrant[]>([]);
	let mountedFolders = $state<MountedFolder[]>([]);
	let isBusy = $state(false);
	let isMountBusy = $state(false);
	let settings = $state<CompanionSettings>(defaultSettings);
	let settingsMessage = $state('');
	let isSavingSettings = $state(false);
	let ollamaModelOptions = $state<string[]>([]);
	let llamaCppModelOptions = $state<string[]>([]);
	let mlxModelOptions = $state<string[]>([]);

	onMount(() => {
		void bootstrap();
		void registerShellEvents();
		const intervalID = window.setInterval(() => {
			void refreshRuntime();
			void refreshGrants();
			void refreshMounts();
		}, 1000);
		return () => window.clearInterval(intervalID);
	});

	async function bootstrap() {
		settings = await loadCompanionSettings();
		await refreshStatus();
		if (status.paired) {
			await ensureRuntime();
		}
		void refreshAvailableModels();
	}

	async function refreshAvailableModels() {
		ollamaModelOptions = await fetchBackendModels('ollama', settings.ollama.baseURL);
		llamaCppModelOptions = await fetchBackendModels('llamacpp', settings.llamacpp.baseURL);
		mlxModelOptions = await fetchBackendModels('mlx', settings.mlx.baseURL);
	}

	function toggleBackendInOrder(name: string) {
		if (settings.localBackendOrder.includes(name)) {
			settings.localBackendOrder = settings.localBackendOrder.filter((entry) => entry !== name);
			return;
		}
		settings.localBackendOrder = [...settings.localBackendOrder, name];
	}

	async function persistSettings() {
		isSavingSettings = true;
		settingsMessage = '';
		try {
			settings = await saveCompanionSettings(settings);
			if (runtime.isRunning) {
				await restartCompanionRuntime();
				runtime = readRuntimeStatus();
			}
			settingsMessage = 'Settings saved.';
			void refreshAvailableModels();
		} catch (errorValue) {
			settingsMessage = errorValue instanceof Error ? errorValue.message : 'Save failed';
		} finally {
			isSavingSettings = false;
		}
	}

	async function refreshStatus() {
		try {
			status = await readCompanionStatus();
			await refreshRuntime();
		} catch {
			status = { paired: false };
			runtime = readRuntimeStatus();
		}
	}

	async function refreshRuntime() {
		runtime = await refreshRuntimeStatus();
	}

	async function refreshGrants() {
		if (!runtime.isRunning) {
			activeGrants = [];
			return;
		}
		try {
			activeGrants = await readActiveGrants();
		} catch {
			activeGrants = [];
		}
	}

	async function refreshMounts() {
		if (!runtime.isRunning) {
			mountedFolders = [];
			return;
		}
		try {
			mountedFolders = await readMountedFolders();
		} catch {
			mountedFolders = [];
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
					denialReason = '';
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
			await refreshGrants();
			await refreshMounts();
		} catch (errorValue) {
			runtime = { isRunning: false, lastError: errorValue instanceof Error ? errorValue.message : 'Runtime failed to start' };
		}
	}

	async function revokeActiveGrant(grantID: string) {
		try {
			await revokeGrant(grantID);
			await refreshGrants();
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Grant revoke failed';
		}
	}

	async function addMount() {
		if (!runtime.isRunning) return;
		isMountBusy = true;
		message = '';
		try {
			const path = await invoke<string | null>('pick_mount_directory');
			if (!path) return;
			await addMountedFolder(path);
			await refreshMounts();
			message = 'Folder mounted.';
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Mount failed';
		} finally {
			isMountBusy = false;
		}
	}

	async function revokeMount(mountID: string) {
		try {
			await revokeMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Mount revoke failed';
		}
	}

	async function pauseMount(mountID: string) {
		try {
			await pauseMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Mount pause failed';
		}
	}

	async function resumeMount(mountID: string) {
		try {
			await resumeMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			message = errorValue instanceof Error ? errorValue.message : 'Mount resume failed';
		}
	}

	async function completePrompt(response: unknown) {
		if (!pendingPrompt) return;
		const requestID = pendingPrompt.requestID;
		try {
			await invoke('complete_prompt_request', { requestId: requestID, response });
			pendingPrompt = undefined;
			promptInput = '';
			denialReason = '';
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
		<div class="runtime-row">
			<span>browser runtime</span>
			<span>{status.browserRuntimeStatus ?? 'unknown'}</span>
		</div>
		<div class="runtime-row">
			<span>last heartbeat</span>
			<span>{runtime.lastHeartbeatAt ? new Date(runtime.lastHeartbeatAt).toLocaleTimeString() : 'none yet'}</span>
		</div>
		{#if runtime.restartAttempts}
			<p class="subtle">Runtime restarted {runtime.restartAttempts} time{runtime.restartAttempts === 1 ? '' : 's'}.</p>
		{/if}
		{#if status.browserRuntimeError}
			<p class="message error">{status.browserRuntimeError}</p>
		{/if}
		{#if runtime.lastError}
			<p class="message error">{runtime.lastError}</p>
		{/if}
	</section>

	<section class="mount-panel">
		<div class="panel-header">
			<h2>Mounted folders</h2>
			<button class="secondary" disabled={!runtime.isRunning || isMountBusy} onclick={addMount}>
				{isMountBusy ? 'Adding...' : 'Add folder'}
			</button>
		</div>
		{#if mountedFolders.length}
			<div class="mount-list">
				{#each mountedFolders as mount}
					<div class="mount-row">
						<div>
							<strong>{mount.displayName}</strong>
							<span>{mount.guestPath}</span>
						</div>
						<div class="mount-actions">
							<span class="mount-badge">{mount.mode}</span>
							<span class:online={mount.status === 'online'} class="mount-badge">{mount.status}</span>
							{#if mount.status === 'online'}
								<button class="secondary" onclick={() => pauseMount(mount.mountID)}>Pause</button>
							{:else}
								<button class="secondary" onclick={() => resumeMount(mount.mountID)}>Resume</button>
							{/if}
							<button class="secondary" onclick={() => revokeMount(mount.mountID)}>Eject</button>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<p class="subtle">Folders you mount here appear to Blueclaw as read/write paths under /workspace/mounts.</p>
		{/if}
	</section>

	<section class="settings-panel">
		<h2>Settings</h2>
		<label class="toggle">
			<input bind:checked={settings.preferCompanionBrowser} type="checkbox" />
			<span>Use this computer for browsing</span>
		</label>
		<label class="toggle">
			<input bind:checked={settings.enableLocalLLM} type="checkbox" />
			<span>Use this computer for AI inference</span>
		</label>
		{#if settings.enableLocalLLM}
			<div class="backend-list">
				<p class="subtle">Backend priority (first available wins)</p>
				{#each ['ollama', 'llamacpp', 'mlx'] as backendName}
					<label class="toggle">
						<input
							checked={settings.localBackendOrder.includes(backendName)}
							onchange={() => toggleBackendInOrder(backendName)}
							type="checkbox"
						/>
						<span>{backendName}</span>
					</label>
				{/each}
			</div>
			<div class="backend-config">
				{#if settings.localBackendOrder.includes('ollama')}
					<h3>Ollama model</h3>
					<input list="ollama-models" bind:value={settings.ollama.model} placeholder="gemma3:1b" />
					<datalist id="ollama-models">
						{#each ollamaModelOptions as modelName}
							<option value={modelName}></option>
						{/each}
					</datalist>
				{/if}
				{#if settings.localBackendOrder.includes('llamacpp')}
					<h3>llama.cpp model</h3>
					<input list="llamacpp-models" bind:value={settings.llamacpp.model} placeholder="default" />
					<datalist id="llamacpp-models">
						{#each llamaCppModelOptions as modelName}
							<option value={modelName}></option>
						{/each}
					</datalist>
				{/if}
				{#if settings.localBackendOrder.includes('mlx')}
					<h3>MLX model</h3>
					<input list="mlx-models" bind:value={settings.mlx.model} placeholder="mlx-community/..." />
					<datalist id="mlx-models">
						{#each mlxModelOptions as modelName}
							<option value={modelName}></option>
						{/each}
					</datalist>
				{/if}
			</div>
			<p class="subtle">Endpoints default to localhost. Edit <code>companion.json</code> for custom URLs.</p>
		{/if}
		<div class="actions">
			<button disabled={isSavingSettings} onclick={persistSettings}>
				{isSavingSettings ? 'Saving...' : 'Save settings'}
			</button>
		</div>
		{#if settingsMessage}
			<p class="message">{settingsMessage}</p>
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
				{:else if pendingPrompt.kind === 'approval'}
					<label>
						<span>Optional reason or constraint</span>
						<input bind:value={denialReason} placeholder="Example: Use another way instead" />
					</label>
					<div class="actions">
						<button onclick={() => completePrompt(approvalResponse(true, ''))}>Allow</button>
						<button class="secondary" onclick={() => completePrompt(approvalResponse(false, denialReason))}>Deny</button>
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

	<section class="grant-panel">
		<h2>Allowed for this task</h2>
		{#if activeGrants.length}
			<div class="grant-list">
				{#each activeGrants as grant}
					<div class="grant-row">
						<div>
							<strong>{grant.displayName}</strong>
							<span>{grant.usedJobs}/{grant.maxJobs} jobs used</span>
						</div>
						<button class="secondary" onclick={() => revokeActiveGrant(grant.grantID)}>Revoke</button>
					</div>
				{/each}
			</div>
		{:else}
			<p class="subtle">Temporary permissions you allow for a task will appear here.</p>
		{/if}
	</section>

	<section class="form-panel">
		<h2>Connect manually</h2>
		<label>
			<span>Device URL</span>
			<input bind:value={deviceURL} placeholder="https://device.example.test" />
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
