import { isSupabaseConfigured, supabase } from '$lib/supabase';
import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
import { createScopedBuzzIdentity } from '$lib/scoped-buzz-identity';
import { readVerifiedCompanyScope } from '$lib/company-session-scope';

const identity = createScopedBuzzIdentity({
	readScope: readVerifiedCompanyScope,
	claim: async (scope) => {
		const { claimCentralBuzzSecret } = await import('$lib/buzz-identity-central-login');
		return claimCentralBuzzSecret(scope);
	},
	restore: buzzIdentity.restoreCentralSecret,
	publish: buzzIdentity.keepCentralSecret,
	hide: buzzIdentity.hideCentralSecret
});

let consumers = 0;

export function ensureCentralBuzzIdentity(): Promise<string | null> {
	return identity.ensure();
}

export function invalidateCentralBuzzIdentity(): void {
	if (!isSupabaseConfigured()) return;
	identity.invalidate();
	if (consumers > 0) queueMicrotask(() => { void identity.ensure(); });
}

export function forgetCentralBuzzIdentity(): void {
	identity.invalidate();
	buzzIdentity.secretHex = null;
}

export function keepCentralBuzzIdentity(): () => void {
	if (!isSupabaseConfigured()) return () => {};
	consumers += 1;
	void identity.ensure();
	return () => { consumers -= 1; };
}

export function watchCentralBuzzIdentity(): () => void {
	if (!isSupabaseConfigured()) return () => {};
	let previousToken: string | undefined;
	const { data } = supabase().auth.onAuthStateChange((event, session) => {
		const token = session?.access_token;
		if (event !== 'INITIAL_SESSION' && (token !== previousToken || event === 'SIGNED_OUT')) {
			identity.invalidate();
			buzzIdentity.secretHex = null;
			if (consumers > 0 && token) queueMicrotask(() => { void identity.ensure(); });
		}
		previousToken = token;
	});
	return () => { data.subscription.unsubscribe(); identity.invalidate(); };
}
