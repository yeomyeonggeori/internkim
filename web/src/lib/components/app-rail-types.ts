import type MailIcon from '@lucide/svelte/icons/mail';

export type AppRailItem = {
	href: string;
	label: string;
	icon: typeof MailIcon;
	badgeCount?: number;
	badgeCountLabel?: string;
	badgeDotLabel?: string;
	badgeArrivals?: number;
};

export type AppRailProfileMenuLabels = {
	account: string;
	logOut: string;
	activeWorkspace: string;
	buzzConnect: string;
};
