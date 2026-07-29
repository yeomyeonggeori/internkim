import type MailIcon from '@lucide/svelte/icons/mail';

export type AppRailItem = {
	href: string;
	label: string;
	icon: typeof MailIcon;
};

export type AppRailProfileMenuLabels = {
	account: string;
	logOut: string;
	activeWorkspace: string;
	buzzConnect: string;
};
