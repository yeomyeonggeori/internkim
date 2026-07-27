<script lang="ts">
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
					data-sveltekit-preload-code="off"
					{...props}
				>
					<Icon />
					<span>{item.label}</span>
				</a>
			{/if}
		{/snippet}
	</Sidebar.MenuButton>
</Sidebar.MenuItem>
