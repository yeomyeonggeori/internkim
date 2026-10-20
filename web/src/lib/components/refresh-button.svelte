<script lang="ts">
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { ComponentProps } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import TooltipIconButton from './tooltip-icon-button.svelte';

	type Props = Omit<ComponentProps<typeof Button>, 'onclick' | 'children'> & {
		label: string;
		refresh: () => unknown;
		showsLabel?: boolean;
	};

	let { label, refresh, showsLabel = false, disabled = false, ...buttonProps }: Props = $props();

	let isRefreshing = $state(false);

	async function refreshOnRequest() {
		isRefreshing = true;
		try {
			await refresh();
		} finally {
			isRefreshing = false;
		}
	}
</script>

{#if showsLabel}
	<Button {...buttonProps} disabled={disabled || isRefreshing} aria-busy={isRefreshing} onclick={refreshOnRequest}>
		{#if isRefreshing}<Spinner data-icon="inline-start" />{:else}<RefreshCwIcon data-icon="inline-start" />{/if}
		{label}
	</Button>
{:else}
	<TooltipIconButton {...buttonProps} {label} disabled={disabled || isRefreshing} aria-busy={isRefreshing} onclick={refreshOnRequest}>
		{#if isRefreshing}<Spinner />{:else}<RefreshCwIcon />{/if}
	</TooltipIconButton>
{/if}
