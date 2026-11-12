import { CachedRead } from '$lib/cached-read.svelte';
import { companyTimeZone } from '$lib/company/company-settings';
import { connectedApps } from '$lib/connected-apps';
import { personalAccessTokens } from '$lib/member/personal-access-tokens';
import { whatTheMessengerNeeds } from '$lib/messenger/messenger-account';
import { haveIConnectedMyMessenger } from '$lib/messenger/messenger-directory';
import { myNotificationSettings } from '$lib/notifications/settings';
import { reachability } from '$lib/notifications/subscribe';
import { isPasskeySupported, listPasskeys } from '$lib/supabase-passkey';
import { isSupabaseConfigured } from '$lib/supabase-session';
import { fetchMyAgentDocument } from './persona-api';
import { visibleSettingsGroups, type SettingsAccess, type SettingsSectionID } from './settings-sections';

export const passkeysRead = new CachedRead(listPasskeys);
export const notificationReachabilityRead = new CachedRead(reachability);
export const notificationSettingsRead = new CachedRead(myNotificationSettings);
export const myAgentDocumentRead = new CachedRead(fetchMyAgentDocument);
export const companyTimeZoneRead = new CachedRead(companyTimeZone);
export const messengerConnectionRead = new CachedRead(haveIConnectedMyMessenger);
export const messengerRequirementRead = new CachedRead(whatTheMessengerNeeds);
export const personalAccessTokensRead = new CachedRead(personalAccessTokens);
export const connectedAppsRead = new CachedRead(connectedApps);

function readsOf(sectionID: SettingsSectionID): CachedRead<unknown>[] {
	switch (sectionID) {
		case 'security':
			return isPasskeySupported() ? [passkeysRead] : [];
		case 'notifications':
			return [notificationReachabilityRead, notificationSettingsRead];
		case 'my-agent':
			return [myAgentDocumentRead, companyTimeZoneRead, messengerConnectionRead, messengerRequirementRead];
		case 'developer':
			return isSupabaseConfigured() ? [personalAccessTokensRead, connectedAppsRead] : [personalAccessTokensRead];
		default:
			return [];
	}
}

function readAhead(sectionID: SettingsSectionID): void {
	for (const cachedRead of readsOf(sectionID)) {
		cachedRead.read().catch((failure: unknown) => {
			console.warn('a settings section could not be read ahead', { sectionID, failure });
		});
	}
}

export function createSettingsReadAhead(): (access: SettingsAccess) => void {
	const readSectionIDs = new Set<SettingsSectionID>();
	return (access) => {
		for (const { sections } of visibleSettingsGroups(access)) {
			for (const { id } of sections) {
				if (readSectionIDs.has(id)) continue;
				readSectionIDs.add(id);
				readAhead(id);
			}
		}
	};
}
