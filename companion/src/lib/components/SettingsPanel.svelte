<script lang="ts">
	import { onMount } from 'svelte';
	import type { CompanionStatus } from '../pairing';
	import { runtimeState, updateRuntimeLocalLLM, type LocalLLMBackendStatus } from '../sidecar';
	import { defaultSettings, fetchBackendModels, loadCompanionSettings, saveCompanionSettings, type CompanionSettings } from '../settings';
	import RemoteModelPanel from './RemoteModelPanel.svelte';

	let { status }: { status: CompanionStatus } = $props();

	let settings = $state<CompanionSettings>(defaultSettings);
	let settingsMessage = $state('');
	let isSavingSettings = $state(false);
	let settingsApplyTimeoutID = $state<number | undefined>();
	let ollamaModelOptions = $state<string[]>([]);
	let llamaCppModelOptions = $state<string[]>([]);
	let mlxModelOptions = $state<string[]>([]);

	onMount(() => {
		void bootstrapSettings();
	});

	async function bootstrapSettings() {
		settings = await loadCompanionSettings();
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
			if (runtimeState.isRunning) {
				await updateRuntimeLocalLLM(settings);
			}
			settingsMessage = runtimeState.isRunning ? 'Settings applied.' : 'Settings saved.';
			void refreshAvailableModels();
		} catch (errorValue) {
			settingsMessage = errorValue instanceof Error ? errorValue.message : 'Save failed';
		} finally {
			isSavingSettings = false;
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
		return runtimeState.localLLM?.backends?.find((backend) => backend.name === backendName);
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
</script>

<section class="settings-panel">
	<div class="panel-header">
		<h2>Settings</h2>
		{#if runtimeState.localLLM?.enabled}
			<span class="badge">Live</span>
		{/if}
	</div>
	<label class="switch-row">
		<input bind:checked={settings.preferCompanionBrowser} onchange={scheduleSettingsApply} type="checkbox" />
		<span>Use this computer for browsing</span>
	</label>
	<RemoteModelPanel {status} />
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
