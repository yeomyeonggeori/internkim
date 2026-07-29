<script lang="ts">
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import AppRailContent from '$lib/components/app-rail-content.svelte';
	import AppRailFooter from '$lib/components/app-rail-footer.svelte';
	import AppRailShell from '$lib/components/app-rail-shell.svelte';
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import { useSidebar } from '$lib/components/ui/sidebar/index.js';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { ConfirmDeleteDialog, confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import AppMobileNavigation from '$lib/components/app-mobile-navigation.svelte';
	import type { AppMobileNavigationItem } from '$lib/components/app-mobile-navigation.svelte';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import ListChecksIcon from '@lucide/svelte/icons/list-checks';
	import MailIcon from '@lucide/svelte/icons/mail';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { onMount } from 'svelte';

	const text = createPageText(appShellText);
	const sidebar = useSidebar();

	const mobilePrimaryItems = $derived<AppMobileNavigationItem[]>([
		{ href: '/mail/', label: text.mail, icon: MailIcon },
		{ href: '/attendance/', label: text.attendance, icon: ClipboardCheckIcon },
		{ href: '/flow/', label: text.flow, icon: ListChecksIcon },
		{ href: '/calendar/', label: text.calendar, icon: CalendarDaysIcon }
	]);

	const mobileMoreItems = $derived<AppMobileNavigationItem[]>([
		{ href: '/memory/', label: text.memory, icon: NetworkIcon },
		{ href: '/organization/', label: text.organization, icon: UsersRoundIcon },
		{ href: '/files/', label: text.files, icon: FolderOpenIcon },
		...appNavigation.workspace
	]);

	const profileMenuLabels = $derived({
		account: text.account,
		logOut: text.logOut,
		activeWorkspace: text.activeWorkspace,
		buzzConnect: text.buzzConnect
	});

	onMount(appNavigation.load);

	function handleKeydown(event: KeyboardEvent) {
		attendanceClock.handleShortcut(event);
		if (!isPlainShortcut(event, 'Comma')) return;
		event.preventDefault();
		sidebar.toggle();
	}

	function requestLogOut() {
		confirmDelete({
			title: text.logOutConfirmTitle,
			description: text.logOutConfirmDescription,
			confirm: { text: text.logOut },
			cancel: { text: text.cancel },
			onConfirm: appNavigation.logOut
		});
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<AppRailShell>
	<AppRailContent
		apps={appNavigation.apps}
		appsLabel={text.apps}
		workspace={appNavigation.workspace}
		workspaceLabel={text.workspace}
		currentPath={appNavigation.currentPath}
	/>
	<AppRailFooter
		contactItem={appNavigation.contactItem}
		displayUserName={appNavigation.displayUserName}
		userEmail={appNavigation.userEmail}
		userImage={appNavigation.userImage}
		labels={profileMenuLabels}
		logOut={requestLogOut}
	/>
</AppRailShell>

<AppMobileNavigation
	displayUserName={appNavigation.displayUserName}
	isActive={appNavigation.isActive}
	logOut={requestLogOut}
	moreItems={mobileMoreItems}
	primaryItems={mobilePrimaryItems}
	{text}
	userEmail={appNavigation.userEmail}
	userImage={appNavigation.userImage}
/>

<ConfirmDeleteDialog />
