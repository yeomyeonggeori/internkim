import { isSupabaseConfigured } from '$lib/supabase';
import { sameBuzzIdentityScope, type BuzzIdentityScope } from '$lib/scoped-buzz-identity';

const STORAGE_KEY = 'internkim.buzz.secret';
const SCOPE_STORAGE_KEY = 'internkim.buzz.scope';

function initialSecret(): string | null {
	if (typeof sessionStorage === 'undefined') return null;
	if (isSupabaseConfigured() || sessionStorage.getItem(SCOPE_STORAGE_KEY)) return null;
	return sessionStorage.getItem(STORAGE_KEY);
}

const store = $state<{ secretHex: string | null; revision: number }>({ secretHex: initialSecret(), revision: 0 });
let activeScope: BuzzIdentityScope | null = null;

// The unlocked signing key is kept in sessionStorage so a page refresh in the
// same tab does not force another unlock. It clears when the tab closes.
export const buzzIdentity = {
	get revision(): number { return store.revision; },
	get secretHex(): string | null {
		return store.secretHex;
	},
	set secretHex(next: string | null) {
		activeScope = null;
		store.revision += 1;
		store.secretHex = next;
		if (typeof sessionStorage === 'undefined') return;
		sessionStorage.removeItem(SCOPE_STORAGE_KEY);
		if (next) {
			sessionStorage.setItem(STORAGE_KEY, next);
		} else {
			sessionStorage.removeItem(STORAGE_KEY);
		}
	},
	hideCentralSecret(): void {
		activeScope = null;
		store.revision += 1;
		store.secretHex = null;
	},
	restoreCentralSecret(scope: BuzzIdentityScope): string | null {
		if (typeof sessionStorage === 'undefined') return null;
		try {
			const saved: BuzzIdentityScope = JSON.parse(sessionStorage.getItem(SCOPE_STORAGE_KEY) ?? 'null');
			if (saved && sameBuzzIdentityScope(saved, scope)) return sessionStorage.getItem(STORAGE_KEY);
		} catch { /* An old or incomplete identity is claimed again. */ }
		return null;
	},
	keepCentralSecret(scope: BuzzIdentityScope, secret: string): void {
		if (activeScope && sameBuzzIdentityScope(activeScope, scope) && store.secretHex === secret) return;
		activeScope = scope;
		store.revision += 1;
		store.secretHex = secret;
		if (typeof sessionStorage === 'undefined') return;
		sessionStorage.setItem(STORAGE_KEY, secret);
		sessionStorage.setItem(SCOPE_STORAGE_KEY, JSON.stringify({
			projectURL: scope.projectURL, accountID: scope.accountID, companyID: scope.companyID, sessionKey: scope.sessionKey
		}));
	}
};
