<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { AppRailItem } from '$lib/components/app-rail-types';

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
	const badgeCount = $derived(item.badgeCount ?? 0);
</script>

<Sidebar.MenuItem>
	<Sidebar.MenuButton isActive={!isExternal && isActive} tooltipContent={item.label}>
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
					data-sveltekit-preload-data="off"
					data-sveltekit-preload-code="viewport"
					{...props}
				>
					<Icon />
					<span>{item.label}</span>
				</a>
			{/if}
		{/snippet}
	</Sidebar.MenuButton>
	{#if badgeCount > 0}
		<Badge
			data-testid="app-rail-nav-badge"
			class="bg-destructive pointer-events-none absolute top-1/2 right-2 h-4 min-w-4 -translate-y-1/2 rounded-full px-1 text-[10px] tabular-nums text-white group-data-[collapsible=icon]:top-0 group-data-[collapsible=icon]:right-0 group-data-[collapsible=icon]:translate-x-1 group-data-[collapsible=icon]:-translate-y-1"
		>
			{badgeCount}
		</Badge>
	{/if}
</Sidebar.MenuItem>
