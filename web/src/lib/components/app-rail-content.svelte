<script lang="ts">
	import AppRailDivider from '$lib/components/app-rail-divider.svelte';
	import AppRailNavSection from '$lib/components/app-rail-nav-section.svelte';
	import type { AppRailItem } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		apps,
		workspace,
		currentPath
	}: {
		apps: AppRailItem[];
		workspace: AppRailItem[];
		currentPath: string;
	} = $props();
</script>

<Sidebar.Content class="overflow-visible">
	<nav class="flex min-h-0 flex-1 flex-col">
		<AppRailNavSection items={apps} {currentPath} />
		<AppRailDivider />
		<AppRailNavSection items={workspace} {currentPath} />
	</nav>
</Sidebar.Content>
