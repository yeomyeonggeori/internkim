<script lang="ts">
	import { isCompanionVerified, type CompanionStatus } from '../pairing';
	import { readRuntimeRemoteModel, updateRuntimeRemoteModel } from '../sidecar';

	let { status }: { status: CompanionStatus } = $props();

	const remoteModelOptions = [
		'google/gemini-3.5-flash',
		'google/gemini-3.1-flash-preview',
		'openai/gpt-5.1',
		'anthropic/claude-sonnet-4.5'
	];

	let remoteModel = $state('');
	let appliedRemoteModel = $state('');
	let remoteModelMessage = $state('');
	let remoteModelApplyTimeoutID = $state<number | undefined>();
	let isApplyingRemoteModel = $state(false);

	$effect(() => {
		if (isCompanionVerified(status)) {
			void refreshRemoteModel();
		} else {
			remoteModel = '';
			appliedRemoteModel = '';
		}
	});

	async function refreshRemoteModel() {
		if (!isCompanionVerified(status)) return;
		try {
			remoteModel = await readRuntimeRemoteModel();
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
			appliedRemoteModel = await updateRuntimeRemoteModel(modelName);
			remoteModel = appliedRemoteModel;
			remoteModelMessage = 'Remote model applied.';
		} catch (errorValue) {
			remoteModelMessage = errorValue instanceof Error ? errorValue.message : 'Remote model update failed';
		} finally {
			isApplyingRemoteModel = false;
		}
	}
</script>

<div class="model-row">
	<div class="model-row-header">
		<div>
			<h3>Remote model</h3>
			<p>Provider model used by the connected agent runtime when execution mode reaches remote.</p>
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
		placeholder="google/gemini-3.5-flash"
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
