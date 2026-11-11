import { companyPathOf, companySlugOf } from '$lib/company-path';
import type MailIcon from '@lucide/svelte/icons/mail';
import BellIcon from '@lucide/svelte/icons/bell';
import BotIcon from '@lucide/svelte/icons/bot';
import Building2Icon from '@lucide/svelte/icons/building-2';
import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
import CodeIcon from '@lucide/svelte/icons/code';
import CreditCardIcon from '@lucide/svelte/icons/credit-card';
import GaugeIcon from '@lucide/svelte/icons/gauge';
import KeyRoundIcon from '@lucide/svelte/icons/key-round';
import MonitorIcon from '@lucide/svelte/icons/monitor';
import SparklesIcon from '@lucide/svelte/icons/sparkles';
import UsersIcon from '@lucide/svelte/icons/users';

export type SettingsGroup = 'general' | 'admin';

export type SettingsSectionID =
	| 'security'
	| 'notifications'
	| 'my-agent'
	| 'developer'
	| 'usage'
	| 'host'
	| 'company'
	| 'members'
	| 'attendance'
	| 'agent'
	| 'billing';

export type SettingsAccess = {
	isSupabaseConfigured: boolean;
	role: 'admin' | 'member' | 'unknown';
	paysBySubscription: boolean | 'unknown';
};

export type SettingsAvailability = 'shown' | 'hidden' | 'unknown';

export type SettingsSection = {
	id: SettingsSectionID;
	group: SettingsGroup;
	icon: typeof MailIcon;
	availability: (access: SettingsAccess) => SettingsAvailability;
};

export type SettingsSectionGroup = { group: SettingsGroup; sections: SettingsSection[] };

const settingsGroups: SettingsGroup[] = ['general', 'admin'];

function always(): SettingsAvailability {
	return 'shown';
}

function whenSupabaseIsConfigured(access: SettingsAccess): SettingsAvailability {
	return access.isSupabaseConfigured ? 'shown' : 'hidden';
}

function whenSubscribed(access: SettingsAccess): SettingsAvailability {
	if (!access.isSupabaseConfigured) return 'hidden';
	if (access.paysBySubscription === 'unknown') return 'unknown';
	return access.paysBySubscription ? 'shown' : 'hidden';
}

function forRole(role: 'admin' | 'member', access: SettingsAccess): SettingsAvailability {
	if (access.role === 'unknown') return 'unknown';
	return access.role === role ? 'shown' : 'hidden';
}

function forAdministrators(access: SettingsAccess): SettingsAvailability {
	return forRole('admin', access);
}

function forAdministratorsWithSupabase(access: SettingsAccess): SettingsAvailability {
	if (!access.isSupabaseConfigured) return 'hidden';
	return forRole('admin', access);
}

function forMembersWithSupabase(access: SettingsAccess): SettingsAvailability {
	if (!access.isSupabaseConfigured) return 'hidden';
	return forRole('member', access);
}

export const settingsSections: SettingsSection[] = [
	{ id: 'security', group: 'general', icon: KeyRoundIcon, availability: always },
	{ id: 'notifications', group: 'general', icon: BellIcon, availability: always },
	{ id: 'my-agent', group: 'general', icon: BotIcon, availability: always },
	{ id: 'developer', group: 'general', icon: CodeIcon, availability: always },
	{ id: 'usage', group: 'general', icon: GaugeIcon, availability: whenSubscribed },
	{ id: 'host', group: 'general', icon: MonitorIcon, availability: forMembersWithSupabase },
	{ id: 'company', group: 'admin', icon: Building2Icon, availability: forAdministrators },
	{ id: 'members', group: 'admin', icon: UsersIcon, availability: forAdministrators },
	{ id: 'attendance', group: 'admin', icon: CalendarClockIcon, availability: forAdministrators },
	{ id: 'agent', group: 'admin', icon: SparklesIcon, availability: forAdministrators },
	{ id: 'billing', group: 'admin', icon: CreditCardIcon, availability: forAdministratorsWithSupabase },
	{ id: 'host', group: 'admin', icon: MonitorIcon, availability: forAdministratorsWithSupabase }
];

export const firstSettingsSectionID: SettingsSectionID = settingsSections[0].id;

export function settingsAccessOf(
	role: { isLoading: boolean; isAdmin: boolean },
	credits: { isLoaded: boolean; paysBySubscription: boolean },
	isSupabaseConfigured: boolean
): SettingsAccess {
	return {
		isSupabaseConfigured,
		role: role.isLoading ? 'unknown' : role.isAdmin ? 'admin' : 'member',
		paysBySubscription: credits.isLoaded ? credits.paysBySubscription : 'unknown'
	};
}

export function visibleSettingsGroups(access: SettingsAccess): SettingsSectionGroup[] {
	return settingsGroups
		.map((group) => ({
			group,
			sections: settingsSections.filter((section) => section.group === group && section.availability(access) === 'shown')
		}))
		.filter((entry) => entry.sections.length > 0);
}

export function settingsSectionAvailability(id: SettingsSectionID, access: SettingsAccess): SettingsAvailability {
	const availabilities = settingsSections.filter((section) => section.id === id).map((section) => section.availability(access));
	if (availabilities.includes('shown')) return 'shown';
	if (availabilities.includes('unknown')) return 'unknown';
	return 'hidden';
}

export function settingsSectionIDOf(routePath: string): SettingsSectionID | null {
	const [, root = '', segment = ''] = routePath.split('/');
	if (root !== 'settings') return null;
	return settingsSections.find((section) => section.id === segment)?.id ?? null;
}

export function settingsPath(pathname: string, sectionID?: SettingsSectionID | 'setup'): string {
	return companyPathOf(companySlugOf(pathname), sectionID ? `/settings/${sectionID}` : '/settings');
}
