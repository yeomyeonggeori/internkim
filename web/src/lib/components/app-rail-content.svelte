<script lang="ts">
	import AppRailNavSection from '$lib/components/app-rail-nav-section.svelte';
	import type { AppRailItem } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		apps,
		appsLabel,
		workspace,
		workspaceLabel,
		currentPath
	}: {
		apps: AppRailItem[];
		appsLabel: string;
		workspace: AppRailItem[];
		workspaceLabel: string;
		currentPath: string;
	} = $props();
</script>

<Sidebar.Content class="pt-2">
	<Sidebar.Group>
		<Sidebar.GroupLabel class="group-data-[collapsible=icon]:pointer-events-none">{appsLabel}</Sidebar.GroupLabel>
		<Sidebar.GroupContent>
			<AppRailNavSection items={apps} {currentPath} />
		</Sidebar.GroupContent>
	</Sidebar.Group>
	{#if workspace.length > 0}
		<Sidebar.Group>
			<Sidebar.GroupLabel class="group-data-[collapsible=icon]:pointer-events-none">{workspaceLabel}</Sidebar.GroupLabel>
			<Sidebar.GroupContent>
				<AppRailNavSection items={workspace} {currentPath} />
			</Sidebar.GroupContent>
		</Sidebar.Group>
	{/if}
</Sidebar.Content>
