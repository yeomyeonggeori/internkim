<script lang="ts">
	import AppRailNavItem from '$lib/components/app-rail-nav-item.svelte';
	import AppRailProfileMenu from '$lib/components/app-rail-profile-menu.svelte';
	import type { AppRailItem, AppRailProfileMenuLabels } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		contactItem,
		displayUserName,
		userEmail,
		userImage,
		labels,
		logOut
	}: {
		contactItem: AppRailItem;
		displayUserName: string;
		userEmail: string;
		userImage?: string;
		labels: AppRailProfileMenuLabels;
		logOut: () => void | Promise<void>;
	} = $props();
</script>

<Sidebar.Footer>
	<Sidebar.Menu>
		<AppRailNavItem item={contactItem} isExternal />
		<AppRailProfileMenu
			{displayUserName}
			{userEmail}
			{userImage}
			{labels}
			{logOut}
		/>
	</Sidebar.Menu>
</Sidebar.Footer>
