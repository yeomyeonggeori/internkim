<script lang="ts">
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import OctagonXIcon from '@lucide/svelte/icons/octagon-x';
	import InfoIcon from '@lucide/svelte/icons/info';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import { mode } from 'mode-watcher';
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from 'svelte-sonner';
	import { MediaQuery } from 'svelte/reactivity';

	let { position = 'bottom-right', offset, mobileOffset, ...restProps }: SonnerProps = $props();
	const isMobile = new MediaQuery('(max-width: 639px)');
	const topOffset = 'calc(var(--app-viewport-top, 0px) + max(16px, env(safe-area-inset-top)))';
</script>

<Sonner
	theme={mode.current}
	class="toaster group"
	position={isMobile.current ? 'top-center' : position}
	mobileOffset={isMobile.current ? { top: topOffset, left: 'max(16px, env(safe-area-inset-left))', right: 'max(16px, env(safe-area-inset-right))' } : mobileOffset}
	offset={isMobile.current ? { top: topOffset } : offset}
	style="--normal-bg: var(--color-popover); --normal-text: var(--color-popover-foreground); --normal-border: var(--color-border);"
	{...restProps}
>
	{#snippet loadingIcon()}
		<Loader2Icon class="size-4 animate-spin" />
	{/snippet}
	{#snippet successIcon()}
		<CircleCheckIcon class="size-4" />
	{/snippet}
	{#snippet errorIcon()}
		<OctagonXIcon class="size-4" />
	{/snippet}
	{#snippet infoIcon()}
		<InfoIcon class="size-4" />
	{/snippet}
	{#snippet warningIcon()}
		<TriangleAlertIcon class="size-4" />
	{/snippet}
</Sonner>
