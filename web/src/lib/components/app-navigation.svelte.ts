import { page } from '$app/state';
import { adminApiFetch } from '$lib/admin-api';
import { appBadgeCounts } from '$lib/components/app-badge-counts.svelte';
import { feedbackFormURL } from '$lib/components/app-rail-config';
import type { AppRailItem } from '$lib/components/app-rail-types';
import { appShellText } from '$lib/i18n/app-shell-text';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { isSupabaseConfigured, signOutOfSupabase, supabaseMember } from '$lib/supabase-session';
import { companyPathOf, routePathOf } from '$lib/company-path';
import { homePath } from '$lib/home-path';
import type { UserRole } from '$lib/types';
import type { WebAuthSession } from '$lib/web-auth-session';
import ActivityIcon from '@lucide/svelte/icons/activity';
import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
import CircleHelpIcon from '@lucide/svelte/icons/circle-help';
import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
import CogIcon from '@lucide/svelte/icons/cog';
import FolderOpenIcon from '@lucide/svelte/icons/folder-open';
import HandshakeIcon from '@lucide/svelte/icons/handshake';
import ListChecksIcon from '@lucide/svelte/icons/list-checks';
import MailIcon from '@lucide/svelte/icons/mail';
import MessagesSquareIcon from '@lucide/svelte/icons/messages-square';
import BrainIcon from '@lucide/svelte/icons/brain';
import NetworkIcon from '@lucide/svelte/icons/network';
import { withReturnPath } from '$lib/return-path';

type AdminSession = {
	email?: string;
	claimedAdminEmail?: string;
	isAdmin?: boolean;
	role?: UserRole;
	canViewTasks?: boolean;
	image?: string;
};

const text = createPageText(appShellText);

class AppNavigation {
	userEmail = $state('');
	userName = $state('');
	userImage = $state('');
	userMemberID = $state('');
	adminRole = $state<UserRole>('member');
	canViewTasks = $state(false);
	companySlug = $state('');

	currentPath = $derived(routePathOf(page.url.pathname));
	link = (path: string) => companyPathOf(this.companySlug, path);
	displayUserName = $derived(this.userName || text.workspace);

	apps = $derived<AppRailItem[]>([
		{ href: this.link('/messenger/'), label: text.messenger, icon: MessagesSquareIcon },
		{ href: this.link('/task/'), label: text.task, icon: ListChecksIcon, badgeCount: appBadgeCounts.requestedTasks },
		{ href: this.link('/memory/'), label: text.memory, icon: BrainIcon },
		{ href: this.link('/calendar/'), label: text.calendar, icon: CalendarDaysIcon, badgeCount: appBadgeCounts.participatingEvents },
		{ href: this.link('/mail/'), label: text.mail, icon: MailIcon },
		{ href: this.link('/attendance/'), label: text.attendance, icon: ClipboardCheckIcon },
		{ href: this.link('/crm/'), label: text.crm, icon: HandshakeIcon },
		{ href: this.link('/organization/'), label: text.organization, icon: NetworkIcon },
		{ href: this.link('/files/'), label: text.files, icon: FolderOpenIcon }
	]);

	workspace = $derived<AppRailItem[]>([
		...(this.canViewTasks
			? [
					{ href: this.link('/runs/'), label: text.tasks, icon: ActivityIcon }
				]
			: []),
		{ href: this.link('/settings/'), label: text.settings, icon: CogIcon }
	]);

	contactItem = $derived<AppRailItem>({ href: feedbackFormURL, label: text.contact, icon: CircleHelpIcon });

	isActive = (href: string) => {
		const base = routePathOf(href).replace(/\/$/, '');
		return this.currentPath === base || this.currentPath.startsWith(`${base}/`);
	};

	load = async (session: WebAuthSession | null) => {
		if (!session?.authenticated) {
			this.clearSession();
			return;
		}
		this.userEmail = session.email;
		this.userName = session.email ? session.email.split('@')[0] : '';
		this.userImage = session.image;
		this.canViewTasks = session.canViewTasks;
		if (isSupabaseConfigured()) {
			const member = await supabaseMember();
			this.userMemberID = member.memberID;
			this.adminRole = member.role;
			this.companySlug = member.companySlug;
			return;
		}
		void appBadgeCounts.load(session.email);
		try {
			const response = await adminApiFetch('/admin/api/session');
			if (!response.ok) {
				this.adminRole = 'member';
				return;
			}
			const session = (await response.json()) as AdminSession;
			this.adminRole = normalizeSessionRole(session);
			this.canViewTasks = session.canViewTasks === true || this.adminRole === 'admin';
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
		if (isSupabaseConfigured()) {
			await signOutOfSupabase();
			location.replace(homePath);
			return;
		}
		let redirectURL = homePath;
		try {
			const response = await fetch(withReturnPath('/auth/logout', this.currentPath), {
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
		this.userMemberID = '';
		this.adminRole = 'member';
		this.canViewTasks = false;
	}

}

function normalizeSessionRole(session: AdminSession): UserRole {
	if (session.role === 'admin' || session.isAdmin) return 'admin';
	return 'member';
}

export const appNavigation = new AppNavigation();
