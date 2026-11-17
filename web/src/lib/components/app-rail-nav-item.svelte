<script lang="ts">
	import { untrack } from 'svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import { appRailMode } from '$lib/components/app-rail-mode.svelte';
	import type { AppRailItem } from '$lib/components/app-rail-types';
	import { unreadBadgeLabel } from '$lib/messenger/unread-badge';

	let {
		item,
		isActive = false,
		isExternal = false
	}: {
		item: AppRailItem;
		isActive?: boolean;
		isExternal?: boolean;
	} = $props();

	const Icon = $derived(item.icon);
	const badgeLabel = $derived(unreadBadgeLabel({ status: 'exact', value: item.badgeCount ?? 0 }));
	const badgeArrivals = $derived(item.badgeArrivals ?? 0);
	let settledArrivals = $state(untrack(() => item.badgeArrivals ?? 0));
	const hasArrived = $derived(badgeArrivals > settledArrivals);

	function settleArrival() {
		settledArrivals = badgeArrivals;
	}
</script>

<Sidebar.MenuItem>
	<Sidebar.MenuButton isActive={!isExternal && isActive} tooltipContent={appRailMode.mode === 'collapsed' ? item.label : undefined}>
		{#snippet child({ props })}
			{#if isExternal}
				<a href={item.href} aria-label={item.label} target="_blank" rel="noopener noreferrer" {...props}>
					<Icon />
					<span>{item.label}</span>
				</a>
			{:else}
				<a
					href={item.href}
					aria-label={item.label}
					data-sveltekit-preload-data="hover"
					{...props}
					class={[props.class, hasArrived && 'has-arrived']}
				>
					{#key badgeArrivals}
						<Icon />
					{/key}
					<span>{item.label}</span>
				</a>
			{/if}
		{/snippet}
	</Sidebar.MenuButton>
	{#if badgeLabel || item.badgeDotLabel}
		{#key badgeArrivals}
			<span
				class={[
					'mark pointer-events-none absolute top-1/2 right-2 flex -translate-y-1/2 group-data-[collapsible=icon]:top-0 group-data-[collapsible=icon]:right-0 group-data-[collapsible=icon]:translate-x-1 group-data-[collapsible=icon]:-translate-y-1',
					hasArrived && 'has-arrived'
				]}
				onanimationend={settleArrival}
			>
				{#if badgeLabel}
					<Badge data-testid="app-rail-nav-badge" class="bg-muted-foreground text-background h-4 min-w-4 rounded-full px-1 text-[10px] tabular-nums">
						{#if item.badgeCountLabel}<span class="sr-only">{item.badgeCountLabel}</span>{/if}
						{badgeLabel}
					</Badge>
				{:else}
					<span data-testid="app-rail-nav-dot" class="bg-muted-foreground m-1 size-2 rounded-full">
						<span class="sr-only">{item.badgeDotLabel}</span>
					</span>
				{/if}
			</span>
		{/key}
	{/if}
</Sidebar.MenuItem>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.mark.has-arrived {
			animation: pop 320ms ease-out;
		}

		:global([data-collapsible='icon']) .mark.has-arrived,
		:global([data-collapsible='icon']) a.has-arrived > :global(svg) {
			animation: hop 420ms ease-out;
		}
	}

	@keyframes pop {
		40% {
			transform: scale(1.35);
		}
	}

	@keyframes hop {
		35% {
			transform: translateY(-5px);
		}
		70% {
			transform: translateY(1px);
		}
	}
</style>
