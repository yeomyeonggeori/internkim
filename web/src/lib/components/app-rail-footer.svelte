<script lang="ts">
	import AppRailHostUpdateItem from '$lib/components/app-rail-host-update-item.svelte';
	import AppRailNavItem from '$lib/components/app-rail-nav-item.svelte';
	import AppRailProfileMenu from '$lib/components/app-rail-profile-menu.svelte';
	import type { AppRailItem, AppRailProfileMenuLabels } from '$lib/components/app-rail-types';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';

	let {
		contactItem,
		hostUpdateLabels,
		displayUserName,
		userEmail,
		userImage,
		userMemberID,
		labels,
		logOut
	}: {
		contactItem: AppRailItem;
		hostUpdateLabels: { label: string; updatingLabel: string };
		displayUserName: string;
		userEmail: string;
		userImage?: string;
		userMemberID?: string;
		labels: AppRailProfileMenuLabels;
		logOut: () => void | Promise<void>;
	} = $props();
</script>

<Sidebar.Footer>
	<Sidebar.Menu>
		<AppRailHostUpdateItem {...hostUpdateLabels} />
		<AppRailNavItem item={contactItem} isExternal />
		<AppRailProfileMenu
			{displayUserName}
			{userEmail}
			{userImage}
			{userMemberID}
			{labels}
			{logOut}
		/>
	</Sidebar.Menu>
</Sidebar.Footer>
