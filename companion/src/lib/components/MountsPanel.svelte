<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { onMount } from 'svelte';
	import {
		addMountedFolder,
		pauseMountedFolder,
		readMountedFolders,
		resumeMountedFolder,
		revokeMountedFolder,
		runtimeState,
		type MountedFolder
	} from '../sidecar';

	let { onMessage }: { onMessage: (text: string) => void } = $props();

	let mountedFolders = $state<MountedFolder[]>([]);
	let isMountBusy = $state(false);

	async function refreshMounts() {
		if (!runtimeState.isRunning) {
			mountedFolders = [];
			return;
		}
		try {
			mountedFolders = await readMountedFolders();
		} catch {
			mountedFolders = [];
		}
	}

	$effect(() => {
		void runtimeState.isRunning;
		void refreshMounts();
	});

	onMount(() => {
		const healthCheckIntervalID = window.setInterval(() => {
			void refreshMounts();
		}, 8000);
		return () => window.clearInterval(healthCheckIntervalID);
	});

	async function addMount() {
		if (!runtimeState.isRunning) return;
		isMountBusy = true;
		onMessage('');
		try {
			const path = await invoke<string | null>('pick_mount_directory');
			if (!path) return;
			await addMountedFolder(path);
			await refreshMounts();
			onMessage('Folder mounted.');
		} catch (errorValue) {
			onMessage(errorValue instanceof Error ? errorValue.message : 'Mount failed');
		} finally {
			isMountBusy = false;
		}
	}

	async function revokeMount(mountID: string) {
		try {
			await revokeMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			onMessage(errorValue instanceof Error ? errorValue.message : 'Mount revoke failed');
		}
	}

	async function pauseMount(mountID: string) {
		try {
			await pauseMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			onMessage(errorValue instanceof Error ? errorValue.message : 'Mount pause failed');
		}
	}

	async function resumeMount(mountID: string) {
		try {
			await resumeMountedFolder(mountID);
			await refreshMounts();
		} catch (errorValue) {
			onMessage(errorValue instanceof Error ? errorValue.message : 'Mount resume failed');
		}
	}
</script>

<section class="mount-panel">
	<div class="panel-header">
		<h2>Mounted folders</h2>
		<button class="secondary" disabled={!runtimeState.isRunning || isMountBusy} onclick={addMount}>
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
		<p class="subtle">Folders you mount here appear to the connected agent workspace under /workspace/mounts.</p>
	{/if}
</section>
