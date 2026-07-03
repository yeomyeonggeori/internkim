<script lang="ts">
	import { onMount } from 'svelte';
	import { readActiveGrants, revokeGrant, runtimeState, type ActiveGrant } from '../sidecar';

	let { onMessage }: { onMessage: (text: string) => void } = $props();

	let activeGrants = $state<ActiveGrant[]>([]);

	async function refreshGrants() {
		if (!runtimeState.isRunning) {
			activeGrants = [];
			return;
		}
		try {
			activeGrants = await readActiveGrants();
		} catch {
			activeGrants = [];
		}
	}

	$effect(() => {
		void runtimeState.isRunning;
		void refreshGrants();
	});

	onMount(() => {
		const healthCheckIntervalID = window.setInterval(() => {
			void refreshGrants();
		}, 8000);
		return () => window.clearInterval(healthCheckIntervalID);
	});

	async function revokeActiveGrant(grantID: string) {
		try {
			await revokeGrant(grantID);
			await refreshGrants();
		} catch (errorValue) {
			onMessage(errorValue instanceof Error ? errorValue.message : 'Grant revoke failed');
		}
	}
</script>

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
