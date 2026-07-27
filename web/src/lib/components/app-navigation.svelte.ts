import { page } from '$app/state';
import { adminApiFetch } from '$lib/admin-api';
import { feedbackFormURL } from '$lib/components/app-rail-config';
import type { AppRailItem } from '$lib/components/app-rail-types';
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
import ComponentIcon from '@lucide/svelte/icons/component';

type WebSession = {
	authenticated?: boolean;
	email?: string;
	image?: string;
	canViewTasks?: boolean;
	isPocSuperAdmin?: boolean;
};

type AdminSession = {
	email?: string;
	claimedAdminEmail?: string;
	isAdmin?: boolean;
	role?: UserRole;
	canViewTasks?: boolean;
	isPocSuperAdmin?: boolean;
	image?: string;
};

const text = createPageText(appShellText);

class AppNavigation {
	userEmail = $state('');
	userName = $state('');
	userImage = $state('');
	adminRole = $state<UserRole>('member');
	canViewTasks = $state(false);
	isPocSuperAdmin = $state(false);

	currentPath = $derived(page.url.pathname);
	displayUserName = $derived(this.userName || text.workspace);

	apps = $derived<AppRailItem[]>([
		{ href: '/flow/', label: text.flow, icon: ListChecksIcon },
		{ href: '/memory/', label: text.memory, icon: NetworkIcon },
		{ href: '/calendar/', label: text.calendar, icon: CalendarDaysIcon },
		{ href: '/mail/', label: text.mail, icon: MailIcon },
		{ href: '/attendance/', label: text.attendance, icon: ClipboardCheckIcon },
		{ href: '/organization/', label: text.organization, icon: ComponentIcon },
		{ href: '/files/', label: text.files, icon: FolderOpenIcon }
	]);

	workspace = $derived<AppRailItem[]>([
		...(this.canViewTasks
			? [
					{
						href: this.isPocSuperAdmin ? '/poc-admin/' : '/tasks/',
						label: this.isPocSuperAdmin ? text.pocAdmin : text.tasks,
						icon: ActivityIcon
					}
				]
			: []),
		{ href: '/settings/', label: text.settings, icon: CogIcon }
	]);

	contactItem = $derived<AppRailItem>({ href: feedbackFormURL, label: text.contact, icon: CircleHelpIcon });

	isActive = (href: string) => {
		const base = href.replace(/\/$/, '');
		return this.currentPath === base || this.currentPath.startsWith(`${base}/`);
	};

	load = async () => {
		const hasWebUser = await this.loadWebSession();
		if (!hasWebUser) {
			this.clearSession();
			return;
		}
		try {
			const response = await adminApiFetch('/admin/api/session');
			if (!response.ok) {
				this.adminRole = 'member';
				return;
			}
			const session = (await response.json()) as AdminSession;
			this.adminRole = normalizeSessionRole(session);
			this.canViewTasks = session.canViewTasks === true || this.adminRole === 'admin';
			this.isPocSuperAdmin = session.isPocSuperAdmin === true;
			const adminEmail = session.email || session.claimedAdminEmail || '';
			if (!adminEmail) {
				this.adminRole = 'member';
				return;
			}
			this.userEmail = adminEmail;
			this.userName = adminEmail.split('@')[0];
			this.userImage = session.image || '';
		} catch {
			this.adminRole = 'member';
		}
	};

	logOut = async () => {
		let redirectURL = '/flow/';
		try {
			const response = await fetch(`/auth/logout?return=${encodeURIComponent(this.currentPath)}`, {
				method: 'POST',
				credentials: 'include'
			});
			if (response.ok) {
				const logoutResponse = (await response.json()) as { redirectURL?: string };
				redirectURL = logoutResponse.redirectURL || redirectURL;
			}
		} finally {
			location.replace(redirectURL);
		}
	};

	private clearSession() {
		this.userEmail = '';
		this.userName = '';
		this.userImage = '';
		this.adminRole = 'member';
		this.canViewTasks = false;
		this.isPocSuperAdmin = false;
	}

	private async loadWebSession() {
		try {
			const response = await fetch(`/auth/session?return=${encodeURIComponent(this.currentPath)}`, {
				credentials: 'include'
			});
			if (!response.ok) {
				this.clearSession();
				return false;
			}
			const session = (await response.json()) as WebSession;
			if (!session.authenticated) {
				this.clearSession();
				return false;
			}
			this.userEmail = session.email || '';
			this.userName = this.userEmail ? this.userEmail.split('@')[0] : '';
			this.userImage = session.image || '';
			this.canViewTasks = session.canViewTasks === true;
			this.isPocSuperAdmin = session.isPocSuperAdmin === true;
			return true;
		} catch {
			this.clearSession();
			return false;
		}
	}
}

function normalizeSessionRole(session: AdminSession): UserRole {
	if (session.role === 'admin' || session.role === 'operationsAdmin') return session.role;
	if (session.isAdmin) return 'admin';
	return 'member';
}

export const appNavigation = new AppNavigation();
