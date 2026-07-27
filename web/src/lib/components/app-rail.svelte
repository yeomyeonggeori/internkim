<script lang="ts">
	import { page } from '$app/state';
	import { adminApiFetch } from '$lib/admin-api';
	import { feedbackFormURL } from '$lib/components/app-rail-config';
	import AppRailContent from '$lib/components/app-rail-content.svelte';
	import AppRailFooter from '$lib/components/app-rail-footer.svelte';
	import AppRailShell from '$lib/components/app-rail-shell.svelte';
	import { attendanceClock } from '$lib/components/attendance-clock.svelte';
	import { useSidebar } from '$lib/components/ui/sidebar/index.js';
	import { isPlainShortcut } from '$lib/keyboard-shortcut';
	import { ConfirmDeleteDialog, confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import type { AppRailItem } from '$lib/components/app-rail-types';
	import AppMobileNavigation from '$lib/components/app-mobile-navigation.svelte';
	import type { AppMobileNavigationItem } from '$lib/components/app-mobile-navigation.svelte';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { UserRole } from '$lib/types';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import CircleHelpIcon from '@lucide/svelte/icons/circle-help';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import CogIcon from '@lucide/svelte/icons/cog';
	import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
	import ListChecksIcon from '@lucide/svelte/icons/list-checks';
	import MailIcon from '@lucide/svelte/icons/mail';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import UsersRoundIcon from '@lucide/svelte/icons/users-round';
	import { onMount } from 'svelte';

	let userEmail = $state('');
	let userImage = $state('');
	const text = createPageText(appShellText);
	const sidebar = useSidebar();
	let adminRole = $state<UserRole>('member');
	let canViewTasks = $state(false);
	let isPocSuperAdmin = $state(false);
	let userName = $state('');
	const currentPath = $derived(page.url.pathname);
	const displayUserName = $derived(userName || text.workspace);

	const apps = $derived<AppRailItem[]>([
		{ href: '/flow/', label: text.flow, icon: ListChecksIcon },
		{ href: '/memory/', label: text.memory, icon: NetworkIcon },
		{ href: '/calendar/', label: text.calendar, icon: CalendarDaysIcon },
		{ href: '/mail/', label: text.mail, icon: MailIcon },
		{ href: '/attendance/', label: text.attendance, icon: ClipboardCheckIcon },
		{ href: '/orgchart/', label: text.orgchart, icon: UsersRoundIcon },
		{ href: '/files/', label: text.files, icon: FolderOpenIcon }
	]);

	const taskRunsItem = $derived<AppRailItem | null>(
		canViewTasks
			? {
					href: isPocSuperAdmin ? '/poc-admin/' : '/tasks/',
					label: isPocSuperAdmin ? text.pocAdmin : text.tasks,
					icon: ActivityIcon
				}
			: null
	);

	const workspace = $derived<AppRailItem[]>([
		...(taskRunsItem ? [taskRunsItem] : []),
		{ href: '/settings/', label: text.settings, icon: CogIcon }
	]);

	const mobilePrimaryItems = $derived<AppMobileNavigationItem[]>([
		{ href: '/mail/', label: text.mail, icon: MailIcon },
		{ href: '/attendance/', label: text.attendance, icon: ClipboardCheckIcon },
		{ href: '/flow/', label: text.flow, icon: ListChecksIcon },
		{ href: '/calendar/', label: text.calendar, icon: CalendarDaysIcon }
	]);

	const mobileMoreItems = $derived<AppMobileNavigationItem[]>([
		{ href: '/memory/', label: text.memory, icon: NetworkIcon },
		{ href: '/orgchart/', label: text.orgchart, icon: UsersRoundIcon },
		{ href: '/files/', label: text.files, icon: FolderOpenIcon },
		...workspace
	]);

	const contactItem = $derived<AppRailItem>({ href: feedbackFormURL, label: text.contact, icon: CircleHelpIcon });
	const profileMenuLabels = $derived({
		account: text.account,
		logOut: text.logOut,
		activeWorkspace: text.activeWorkspace
	});

	onMount(loadUser);

	function handleKeydown(event: KeyboardEvent) {
		attendanceClock.handleShortcut(event);
		if (!isPlainShortcut(event, 'Comma')) return;
		event.preventDefault();
		sidebar.toggle();
	}

	function isActive(href: string) {
		const base = href.replace(/\/$/, '');
		return currentPath === base || currentPath.startsWith(`${base}/`);
	}

	function normalizeSessionRole(session: { role?: UserRole; isAdmin?: boolean }) {
		if (session.role === 'admin' || session.role === 'operationsAdmin') return session.role;
		if (session.isAdmin) return 'admin';
		return 'member';
	}

	async function loadUser() {
		const hasWebUser = await loadWebUser();
		if (!hasWebUser) {
			adminRole = 'member';
			canViewTasks = false;
			isPocSuperAdmin = false;
			userImage = '';
			return;
		}
		try {
			const response = await adminApiFetch('/admin/api/session');
			if (!response.ok) {
				adminRole = 'member';
				return;
			}
			const session = (await response.json()) as {
				email?: string;
				claimedAdminEmail?: string;
				isAdmin?: boolean;
				role?: UserRole;
				canViewTasks?: boolean;
				isPocSuperAdmin?: boolean;
				image?: string;
			};
			adminRole = normalizeSessionRole(session);
			canViewTasks = session.canViewTasks === true || adminRole === 'admin';
			isPocSuperAdmin = session.isPocSuperAdmin === true;
			const adminEmail = session.email || session.claimedAdminEmail || '';
			if (!adminEmail) {
				adminRole = 'member';
				return;
			}
			userEmail = adminEmail;
			userName = userEmail.split('@')[0];
			userImage = session.image || '';
		} catch {
			adminRole = 'member';
		}
	}

	async function loadWebUser() {
		try {
			const response = await fetch(`/auth/session?return=${encodeURIComponent(currentPath)}`, { credentials: 'include' });
			if (!response.ok) {
				userEmail = '';
				userName = '';
				userImage = '';
				canViewTasks = false;
				isPocSuperAdmin = false;
				return false;
			}
			const session = (await response.json()) as {
				authenticated?: boolean;
				email?: string;
				image?: string;
				canViewTasks?: boolean;
				isPocSuperAdmin?: boolean;
			};
			if (!session.authenticated) {
				userEmail = '';
				userName = '';
				userImage = '';
				canViewTasks = false;
				isPocSuperAdmin = false;
				return false;
			}
			userEmail = session.email || '';
			userName = userEmail ? userEmail.split('@')[0] : '';
			userImage = session.image || '';
			canViewTasks = session.canViewTasks === true;
			isPocSuperAdmin = session.isPocSuperAdmin === true;
			return true;
		} catch {
			userEmail = '';
			userName = '';
			userImage = '';
			canViewTasks = false;
			isPocSuperAdmin = false;
			return false;
		}
	}

	function requestLogOut() {
		confirmDelete({
			title: text.logOutConfirmTitle,
			description: text.logOutConfirmDescription,
			confirm: { text: text.logOut },
			cancel: { text: text.cancel },
			onConfirm: logOut
		});
	}

	async function logOut() {
		let redirectURL = '/flow/';
		try {
			const response = await fetch(`/auth/logout?return=${encodeURIComponent(currentPath)}`, { method: 'POST', credentials: 'include' });
			if (response.ok) {
				const logoutResponse = (await response.json()) as { redirectURL?: string };
				redirectURL = logoutResponse.redirectURL || redirectURL;
			}
		} finally {
			location.replace(redirectURL);
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<AppRailShell>
	<AppRailContent {apps} appsLabel={text.apps} {workspace} workspaceLabel={text.workspace} {currentPath} />
	<AppRailFooter
		contactItem={contactItem}
		{displayUserName}
		{userEmail}
		{userImage}
		labels={profileMenuLabels}
		logOut={requestLogOut}
	/>
</AppRailShell>

<AppMobileNavigation
	displayUserName={displayUserName}
	{isActive}
	logOut={requestLogOut}
	moreItems={mobileMoreItems}
	primaryItems={mobilePrimaryItems}
	{text}
	{userEmail}
	{userImage}
/>

<ConfirmDeleteDialog />
