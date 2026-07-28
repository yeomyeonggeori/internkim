<script lang="ts">
	import AppRailNavItem from '$lib/components/app-rail-nav-item.svelte';
	import type { AppRailItem } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		items,
		currentPath
	}: {
		items: AppRailItem[];
		currentPath: string;
	} = $props();

	function isActivePath(href: string) {
		const base = href.replace(/\/$/, '');
		return currentPath === base || currentPath.startsWith(`${base}/`);
	}
</script>

<Sidebar.Menu>
	{#each items as item (item.href)}
		<AppRailNavItem {item} isActive={isActivePath(item.href)} />
	{/each}
</Sidebar.Menu>
