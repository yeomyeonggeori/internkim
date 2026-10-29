<script lang="ts">
	import CircleArrowUpIcon from '@lucide/svelte/icons/circle-arrow-up';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { Spinner } from '$lib/components/ui/spinner';
	import { hostUpdate } from '$lib/host/host-update.svelte';

	let { label, updatingLabel }: { label: string; updatingLabel: string } = $props();

	const release = $derived(hostUpdate.availableRelease);
	const watch = $derived(hostUpdate.watch);
	const version = $derived(watch?.toVersion ?? release?.version ?? '');
	const shownLabel = $derived(watch ? updatingLabel : label);
</script>

{#if release || watch}
	<Sidebar.MenuItem>
		<Sidebar.MenuButton
			tooltipContent={`${shownLabel} ${version}`}
			aria-label={`${shownLabel} ${version}`}
			aria-disabled={watch !== null}
			onclick={() => release && !watch && hostUpdate.confirm(release, false)}
		>
			{#if watch}<Spinner />{:else}<CircleArrowUpIcon class="text-brand" />{/if}
			<span>{shownLabel}</span>
		</Sidebar.MenuButton>
		<Sidebar.MenuBadge class="font-normal text-muted-foreground">{version}</Sidebar.MenuBadge>
	</Sidebar.MenuItem>
{/if}
