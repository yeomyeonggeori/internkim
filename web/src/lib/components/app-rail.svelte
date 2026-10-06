<script lang="ts">
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import AppRailContent from '$lib/components/app-rail-content.svelte';
	import AppRailFooter from '$lib/components/app-rail-footer.svelte';
	import AppRailShell from '$lib/components/app-rail-shell.svelte';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { useSidebar } from '$lib/components/ui/sidebar/index.js';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { ConfirmDeleteDialog, confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import AppMobileNavigation from '$lib/components/app-mobile-navigation.svelte';
	import type { AppMobileNavigationItem } from '$lib/components/app-mobile-navigation.svelte';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { routePathOf } from '$lib/company-path';
	import type { WebAuthSession } from '$lib/web-auth-session';

	let { session, onSearch, workspaceScope = '' }: { session: WebAuthSession | null; onSearch?: () => void; workspaceScope?: string } = $props();

	const text = createPageText(appShellText);
	const sidebar = useSidebar();

	function mobileItems(paths: string[]): AppMobileNavigationItem[] {
		return paths.flatMap(path => appNavigation.apps.filter(item => routePathOf(item.href).replace(/\/$/, '') === path));
	}
	const mobilePrimaryItems = $derived(mobileItems(['/messenger', '/attendance', '/task', '/calendar']));

	const mobileMoreItems = $derived<AppMobileNavigationItem[]>([
		...mobileItems(['/mail', '/memory', '/crm', '/organization', '/files']),
		...appNavigation.workspace
	]);

	const profileMenuLabels = $derived({
		account: text.account,
		logOut: text.logOut,
		activeWorkspace: text.activeWorkspace,
		buzzConnect: text.buzzConnect
	});

	let appliedSessionKey = '';

	$effect(() => {
		const sessionKey = workspaceScope || `${session?.authenticated ?? false}:${session?.email ?? ''}`;
		if (sessionKey === appliedSessionKey) return;
		appliedSessionKey = sessionKey;
		void appNavigation.load(session);
	});

	function handleKeydown(event: KeyboardEvent) {
		myAttendanceToday.handleShortcut(event);
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
		userMemberID={appNavigation.userMemberID}
		labels={profileMenuLabels}
		logOut={requestLogOut}
	/>
</AppRailShell>

<AppMobileNavigation
	{onSearch}
	displayUserName={appNavigation.displayUserName}
	isActive={appNavigation.isActive}
	logOut={requestLogOut}
	moreItems={mobileMoreItems}
	primaryItems={mobilePrimaryItems}
	{text}
	userEmail={appNavigation.userEmail}
	userImage={appNavigation.userImage}
		userMemberID={appNavigation.userMemberID}
/>

<ConfirmDeleteDialog />
