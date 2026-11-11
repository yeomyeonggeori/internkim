import { getContext, setContext } from 'svelte';
import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';

const settingsRoleKey = Symbol('settings-role');

function isAdminSession(value: unknown): boolean {
	return typeof value === 'object' && value !== null && 'isAdmin' in value && value.isAdmin === true;
}

export class SettingsRole {
	isAdmin = $state(false);
	isLoading = $state(true);

	load = async () => {
		if (!isSupabaseConfigured()) {
			const response = await fetch('/admin/api/session', { credentials: 'include' });
			this.isAdmin = response.ok && isAdminSession(await response.json());
			this.isLoading = false;
			return;
		}
		this.isAdmin = (await supabaseMemberRole()) === 'admin';
		this.isLoading = false;
	};
}

export function provideSettingsRole(): SettingsRole {
	return setContext(settingsRoleKey, new SettingsRole());
}

export function settingsRole(): SettingsRole {
	return getContext<SettingsRole>(settingsRoleKey);
}
