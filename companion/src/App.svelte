<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { listen } from '@tauri-apps/api/event';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { getCurrent, onOpenUrl } from '@tauri-apps/plugin-deep-link';
	import { onMount } from 'svelte';
	import { isCompanionVerified, isStalePairingStatus, normalizeManualPairingInput, parsePairingLink, stalePairingMessage, statusLabel, type CompanionStatus } from './lib/pairing';
	import { inputResponse, normalizePromptRequest, type PromptRequest } from './lib/prompts';
	import { addMountedFolder, pairCompanion, pauseMountedFolder, readActiveGrants, readCompanionStatus, readMountedFolders, readRemoteModel, readRuntimeStatus, refreshRuntimeStatus, restartCompanionRuntime, resumeMountedFolder, revokeGrant, revokeMountedFolder, startCompanionRuntime, updateRemoteModel, updateRuntimeLocalLLM, type ActiveGrant, type LocalLLMBackendStatus, type MountedFolder, type RuntimeStatus } from './lib/sidecar';
	import { defaultSettings, fetchBackendModels, loadCompanionSettings, saveCompanionSettings, type CompanionSettings } from './lib/settings';

	let status = $state<CompanionStatus>({ paired: false });
	let runtime = $state<RuntimeStatus>({ isRunning: false });
	let deviceURL = $state('');
	let pairingCode = $state('');
	let message = $state('');
	let pendingPrompt = $state<PromptRequest | undefined>();
	let activeGrants = $state<ActiveGrant[]>([]);
	let mountedFolders = $state<MountedFolder[]>([]);
	let isBusy = $state(false);
	let isMountBusy = $state(false);
	let settings = $state<CompanionSettings>(defaultSettings);
	let settingsMessage = $state('');
	let isSavingSettings = $state(false);
	let settingsApplyTimeoutID = $state<number | undefined>();
	let promptTimeoutID = $state<number | undefined>();
	let remoteModel = $state('');
	let appliedRemoteModel = $state('');
	let remoteModelMessage = $state('');
	let remoteModelApplyTimeoutID = $state<number | undefined>();
	let isApplyingRemoteModel = $state(false);
	let ollamaModelOptions = $state<string[]>([]);
	let llamaCppModelOptions = $state<string[]>([]);
	let mlxModelOptions = $state<string[]>([]);
	const remoteModelOptions = [
		'google/gemini-3.1-flash-lite-preview',
		'google/gemini-3.1-flash-preview',
		'openai/gpt-5.1',
		'anthropic/claude-sonnet-4.5'
	];

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
		if (isCompanionVerified(status)) {
			await ensureRuntime();
			await refreshRemoteModel();
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
			scheduleSettingsApply();
			return;
		}
		settings.localBackendOrder = [...settings.localBackendOrder, name];
		scheduleSettingsApply();
	}

	function scheduleSettingsApply() {
		settingsMessage = '';
		if (settingsApplyTimeoutID) {
			window.clearTimeout(settingsApplyTimeoutID);
		}
		settingsApplyTimeoutID = window.setTimeout(() => {
			void persistSettings();
		}, 250);
	}

	async function persistSettings() {
		isSavingSettings = true;
		settingsMessage = '';
		try {
			settings = await saveCompanionSettings(settings);
			if (runtime.isRunning) {
				const localLLM = await updateRuntimeLocalLLM(settings);
				runtime = { ...readRuntimeStatus(), localLLM };
			}
			settingsMessage = runtime.isRunning ? 'Settings applied.' : 'Settings saved.';
			void refreshAvailableModels();
		} catch (errorValue) {
			settingsMessage = errorValue instanceof Error ? errorValue.message : 'Save failed';
		} finally {
			isSavingSettings = false;
		}
	}

	async function refreshRemoteModel() {
		if (!isCompanionVerified(status)) return;
		try {
			remoteModel = await readRemoteModel();
			appliedRemoteModel = remoteModel;
		} catch (errorValue) {
			remoteModelMessage = errorValue instanceof Error ? errorValue.message : 'Remote model read failed';
		}
	}

	function scheduleRemoteModelApply() {
		remoteModelMessage = '';
		if (remoteModelApplyTimeoutID) {
			window.clearTimeout(remoteModelApplyTimeoutID);
		}
		remoteModelApplyTimeoutID = window.setTimeout(() => {
			void persistRemoteModel();
		}, 800);
	}

	async function persistRemoteModel() {
		const modelName = remoteModel.trim();
		if (!modelName || modelName === appliedRemoteModel || !isCompanionVerified(status)) return;
		isApplyingRemoteModel = true;
		remoteModelMessage = '';
		try {
			appliedRemoteModel = await updateRemoteModel(modelName);
			remoteModel = appliedRemoteModel;
			remoteModelMessage = 'Remote model applied.';
		} catch (errorValue) {
			remoteModelMessage = errorValue instanceof Error ? errorValue.message : 'Remote model update failed';
		} finally {
			isApplyingRemoteModel = false;
		}
	}

	function modelOptionsForBackend(backendName: string): string[] {
		if (backendName === 'ollama') return ollamaModelOptions;
		if (backendName === 'llamacpp') return llamaCppModelOptions;
		if (backendName === 'mlx') return mlxModelOptions;
		return [];
	}

	function endpointForBackend(backendName: string) {
		if (backendName === 'ollama') return settings.ollama;
		if (backendName === 'llamacpp') return settings.llamacpp;
		return settings.mlx;
	}

	function backendStatusFor(backendName: string): LocalLLMBackendStatus | undefined {
		return runtime.localLLM?.backends?.find((backend) => backend.name === backendName);
	}

	function backendLabel(backendName: string): string {
		if (backendName === 'llamacpp') return 'llama.cpp';
		if (backendName === 'mlx') return 'MLX';
		return 'Ollama';
	}

	function backendPlaceholder(backendName: string): string {
		if (backendName === 'ollama') return 'gemma3:1b';
		if (backendName === 'llamacpp') return 'default';
		return 'mlx-community/...';
	}

	async function refreshStatus() {
		try {
			status = await readCompanionStatus();
			if (isStalePairingStatus(status)) {
				message = stalePairingMessage;
			} else if (message === stalePairingMessage) {
				message = '';
			}
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
					showPrompt(normalizePromptRequest(event.payload));
					await invoke('show_main_window');
				} catch (errorValue) {
					message = errorValue instanceof Error ? errorValue.message : 'Prompt request failed';
				}
			});
		} catch {
			message = 'Companion shell events are unavailable.';
		}
	}

	function showPrompt(prompt: PromptRequest) {
		clearPromptTimeout();
		pendingPrompt = prompt;
		if (prompt.kind === 'input') {
			void completePrompt(inputResponse(window.prompt(prompt.message, '') ?? ''));
			return;
		}
		if (!prompt.timeoutSeconds) return;
		promptTimeoutID = window.setTimeout(() => {
			if (pendingPrompt?.requestID !== prompt.requestID) return;
			pendingPrompt = undefined;
		}, prompt.timeoutSeconds * 1000);
	}

	function clearPromptTimeout() {
		if (promptTimeoutID === undefined) return;
		window.clearTimeout(promptTimeoutID);
		promptTimeoutID = undefined;
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
				await refreshRemoteModel();
			}
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

	async function restartRuntime() {
		try {
			await restartCompanionRuntime();
			runtime = readRuntimeStatus();
			await refreshGrants();
			await refreshMounts();
		} catch (errorValue) {
			runtime = { isRunning: false, lastError: errorValue instanceof Error ? errorValue.message : 'Runtime failed to restart' };
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
		clearPromptTimeout();
		try {
			await invoke('complete_prompt_request', { requestId: requestID, response });
			pendingPrompt = undefined;
		} catch (errorValue) {
			pendingPrompt = undefined;
			message = errorValue instanceof Error ? errorValue.message : 'Response failed';
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

	<section class="runtime-panel">
		<h2>Runtime</h2>
		<div class="runtime-row">
			<span>{runtime.isRunning ? `running${runtime.processID ? ` #${runtime.processID}` : ''}` : 'stopped'}</span>
			<button class="secondary" disabled={!isCompanionVerified(status) || runtime.isRunning} onclick={ensureRuntime}>Start</button>
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
		<div class="panel-header">
			<h2>Settings</h2>
			{#if runtime.localLLM?.enabled}
				<span class="badge">Live</span>
			{/if}
		</div>
		<label class="switch-row">
			<input bind:checked={settings.preferCompanionBrowser} onchange={scheduleSettingsApply} type="checkbox" />
			<span>Use this computer for browsing</span>
		</label>
		<div class="model-row">
			<div class="model-row-header">
				<div>
					<h3>Remote model</h3>
					<p>OpenRouter model used by Blueclaw when execution mode reaches remote.</p>
				</div>
				<span class:online={remoteModel === appliedRemoteModel && remoteModel !== ''} class="badge">
					{isApplyingRemoteModel ? 'Applying' : remoteModel === appliedRemoteModel && remoteModel !== '' ? 'Live' : 'Pending'}
				</span>
			</div>
			<input
				bind:value={remoteModel}
				disabled={!isCompanionVerified(status)}
				list="remote-models"
				oninput={scheduleRemoteModelApply}
				placeholder="google/gemini-3.1-flash-lite-preview"
			/>
			<datalist id="remote-models">
				{#each remoteModelOptions as modelName}
					<option value={modelName}></option>
				{/each}
			</datalist>
			<div class="actions">
				<button class="secondary" disabled={!isCompanionVerified(status) || isApplyingRemoteModel || remoteModel.trim() === appliedRemoteModel} onclick={persistRemoteModel}>
					{isApplyingRemoteModel ? 'Applying...' : 'Apply now'}
				</button>
				<button class="ghost" disabled={!isCompanionVerified(status)} onclick={refreshRemoteModel}>Refresh</button>
			</div>
			{#if remoteModelMessage}
				<p class="message">{remoteModelMessage}</p>
			{/if}
		</div>
		<label class="switch-row">
			<input bind:checked={settings.enableLocalLLM} onchange={scheduleSettingsApply} type="checkbox" />
			<span>Use this computer for AI inference</span>
		</label>
		{#if settings.enableLocalLLM}
			<div class="backend-list">
				<p class="subtle">Backend priority</p>
				{#each ['ollama', 'llamacpp', 'mlx'] as backendName}
					<label class="switch-row">
						<input
							checked={settings.localBackendOrder.includes(backendName)}
							onchange={() => toggleBackendInOrder(backendName)}
							type="checkbox"
						/>
						<span>{backendLabel(backendName)}</span>
					</label>
				{/each}
			</div>
			<div class="backend-config">
				{#each settings.localBackendOrder as backendName}
					{@const endpoint = endpointForBackend(backendName)}
					{@const modelOptions = modelOptionsForBackend(backendName)}
					{@const backendStatus = backendStatusFor(backendName)}
					<div class="model-row">
						<div class="model-row-header">
							<div>
								<h3>{backendLabel(backendName)}</h3>
								<p>{endpoint.baseURL}</p>
							</div>
							<span class:online={backendStatus?.available} class="badge">
								{backendStatus?.available ? 'Ready' : 'Idle'}
							</span>
						</div>
						{#if modelOptions.length}
							<select bind:value={endpoint.model} onchange={scheduleSettingsApply}>
								<option value="">Default model</option>
								{#each modelOptions as modelName}
									<option value={modelName}>{modelName}</option>
								{/each}
							</select>
						{:else}
							<input bind:value={endpoint.model} oninput={scheduleSettingsApply} placeholder={backendPlaceholder(backendName)} />
						{/if}
						{#if backendStatus?.lastError}
							<p class="message error">{backendStatus.lastError}</p>
						{/if}
					</div>
				{/each}
			</div>
			<p class="subtle">Model changes apply to the running runtime immediately. Edit <code>companion.json</code> for custom endpoints.</p>
		{/if}
		<div class="actions">
			<button disabled={isSavingSettings} onclick={persistSettings}>
				{isSavingSettings ? 'Applying...' : 'Save settings'}
			</button>
		</div>
		{#if settingsMessage}
			<p class="message">{settingsMessage}</p>
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
