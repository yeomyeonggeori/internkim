<script lang="ts">
	import DownloadIcon from '@lucide/svelte/icons/download';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { Spinner } from '$lib/components/ui/spinner';
	import { appRailMode } from '$lib/components/app-rail-mode.svelte';
	import { tauriAppUpdate } from '$lib/native-shell/app-update-tauri';
	import { appUpdate } from '$lib/native-shell/app-update.svelte';
	import { shellPlatform } from '$lib/native-shell/shell';

	let { label, updatingLabel, failedLabel }: { label: string; updatingLabel: string; failedLabel: string } = $props();

	const update = $derived(appUpdate.state.available);
	const isInstalling = $derived(appUpdate.state.isInstalling);
	const shownLabel = $derived(isInstalling ? updatingLabel : label);

	onMount(() => appUpdate.watch(shellPlatform(), tauriAppUpdate));

	async function install() {
		if ((await appUpdate.install()) === 'failed') toast.error(failedLabel);
	}
</script>

{#if update}
	<Sidebar.MenuItem>
		<Sidebar.MenuButton
			tooltipContent={appRailMode.mode === 'collapsed' ? `${shownLabel} ${update.version}` : undefined}
			aria-label={`${shownLabel} ${update.version}`}
			aria-disabled={isInstalling}
			onclick={install}
		>
			{#if isInstalling}<Spinner />{:else}<DownloadIcon class="text-brand" />{/if}
			<span>{shownLabel}</span>
		</Sidebar.MenuButton>
		<Sidebar.MenuBadge class="font-normal text-muted-foreground">{update.version}</Sidebar.MenuBadge>
	</Sidebar.MenuItem>
{/if}
