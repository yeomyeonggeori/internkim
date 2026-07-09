<script lang="ts">
	import AppRailNavItem from '$lib/components/app-rail-nav-item.svelte';
	import AppRailProfileMenu from '$lib/components/app-rail-profile-menu.svelte';
	import type { AppRailItem, AppRailProfileMenuLabels } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		contactItem,
		profileMenuOpen = $bindable(false),
		displayUserName,
		userEmail,
		userImage,
		labels,
		openAPITokenSheet,
		logOut
	}: {
		contactItem: AppRailItem;
		profileMenuOpen?: boolean;
		displayUserName: string;
		userEmail: string;
		userImage?: string;
		labels: AppRailProfileMenuLabels;
		openAPITokenSheet: () => void;
		logOut: () => void | Promise<void>;
	} = $props();
</script>

<Sidebar.Footer class="gap-1 p-0">
	<Sidebar.Menu class="gap-1">
		<AppRailNavItem item={contactItem} isExternal />
	</Sidebar.Menu>

	<AppRailProfileMenu
		bind:open={profileMenuOpen}
		{displayUserName}
		{userEmail}
		{userImage}
		{labels}
		{openAPITokenSheet}
		{logOut}
	/>
</Sidebar.Footer>
