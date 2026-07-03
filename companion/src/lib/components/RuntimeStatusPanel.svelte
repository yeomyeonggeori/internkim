<script lang="ts">
	import { onMount } from 'svelte';
	import { isCompanionVerified, type CompanionStatus } from '../pairing';
	import { refreshRuntimeStatus, runtimeState } from '../sidecar';

	let { status, onStart }: { status: CompanionStatus; onStart: () => void } = $props();

	onMount(() => {
		const healthCheckIntervalID = window.setInterval(() => {
			void refreshRuntimeStatus();
		}, 8000);
		return () => window.clearInterval(healthCheckIntervalID);
	});
</script>

<section class="runtime-panel">
	<h2>Runtime</h2>
	<div class="runtime-row">
		<span>{runtimeState.isRunning ? `running${runtimeState.processID ? ` #${runtimeState.processID}` : ''}` : 'stopped'}</span>
		<button class="secondary" disabled={!isCompanionVerified(status) || runtimeState.isRunning} onclick={onStart}>Start</button>
	</div>
	<div class="runtime-row">
		<span>browser runtime</span>
		<span>{status.browserRuntimeStatus ?? 'unknown'}</span>
	</div>
	<div class="runtime-row">
		<span>last heartbeat</span>
		<span>{runtimeState.lastHeartbeatAt ? new Date(runtimeState.lastHeartbeatAt).toLocaleTimeString() : 'none yet'}</span>
	</div>
	{#if runtimeState.restartAttempts}
		<p class="subtle">Runtime restarted {runtimeState.restartAttempts} time{runtimeState.restartAttempts === 1 ? '' : 's'}.</p>
	{/if}
	{#if status.browserRuntimeError}
		<p class="message error">{status.browserRuntimeError}</p>
	{/if}
	{#if runtimeState.lastError}
		<p class="message error">{runtimeState.lastError}</p>
	{/if}
</section>
