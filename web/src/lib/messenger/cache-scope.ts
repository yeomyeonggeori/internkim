import { isSupabaseConfigured, projectURL, supabase } from '$lib/supabase';
import { readVerifiedCompanyScope } from '$lib/company-session-scope';

export type MessengerCacheScope = { key: string; generation: number };

let generation = 0;
let active: MessengerCacheScope | null = null;
let token = '';
let project = '';
let accountID = '';
let adopted = false;
let reading: Promise<MessengerCacheScope> | null = null;
let watching = false;
const resets = new Set<() => void>();
const storagePrefix = 'messenger-conversations:';

export function onMessengerCacheReset(reset: () => void): () => void {
	resets.add(reset);
	return () => { resets.delete(reset); };
}

export function invalidateMessengerCacheScope(forgetStored = false): void {
	reading = null;
	if (!forgetStored) return;
	generation += 1;
	active = null;
	try {
		if (typeof sessionStorage !== 'undefined') {
			for (const key of Object.keys(sessionStorage)) {
				if (key === 'messenger-conversations' || key.startsWith(storagePrefix)) sessionStorage.removeItem(key);
			}
		}
	} catch { /* Storage can be unavailable while in-memory caches still need clearing. */ }
	try {
		if (typeof localStorage !== 'undefined') {
			for (const key of Object.keys(localStorage)) {
				if (key === 'messenger-last-channel' || key.startsWith('messenger-last-channel:')) localStorage.removeItem(key);
			}
		}
	} catch { /* A storage refusal must not retain the previous account on screen. */ }
	for (const reset of resets) reset();
}

export function messengerCacheKey(): string {
	return isSupabaseConfigured() ? active?.key ?? '' : 'device';
}

export function isCurrentMessengerScope(scope: MessengerCacheScope): boolean {
	return scope.generation === generation && scope.key === messengerCacheKey();
}

export async function requireCurrentMessengerScope(scope: MessengerCacheScope): Promise<void> {
	if (isSupabaseConfigured()) {
		const { data } = await supabase().auth.getSession();
		if (data.session?.access_token !== token || projectURL() !== project) throw new Error('the messenger account changed');
	}
	if (!isCurrentMessengerScope(scope)) throw new Error('the messenger account changed');
}

export async function messengerCacheScope(): Promise<MessengerCacheScope> {
	if (!isSupabaseConfigured()) return { key: 'device', generation };
	const client = supabase();
	if (!watching) {
		watching = true;
		client.auth.onAuthStateChange((event, session) => {
			const nextToken = session?.access_token ?? '';
			const nextAccount = session?.user.id ?? '';
			const ownerChanged = adopted && accountID !== nextAccount;
			const tokenChanged = token !== nextToken;
			token = nextToken;
			accountID = nextAccount;
			if (ownerChanged || event === 'SIGNED_OUT') invalidateMessengerCacheScope(true);
			else if (adopted && tokenChanged) invalidateMessengerCacheScope();
		});
	}
	const { data } = await client.auth.getSession();
	const nextToken = data.session?.access_token ?? '';
	const nextProject = projectURL();
	const nextAccount = data.session?.user.id ?? '';
	const ownerChanged = adopted && (accountID !== nextAccount || project !== nextProject);
	const tokenChanged = token !== nextToken;
	token = nextToken;
	project = nextProject;
	accountID = nextAccount;
	if (ownerChanged) invalidateMessengerCacheScope(true);
	else if (adopted && tokenChanged) invalidateMessengerCacheScope();
	adopted = true;
	if (!token) throw new Error('sign in first');
	if (reading) return reading;
	const startedGeneration = generation;
	const startedToken = token;
	const attempt = readVerifiedCompanyScope().then(async (scope) => {
		if (!scope || scope.accessToken !== startedToken || token !== startedToken || generation !== startedGeneration || scope.projectURL !== project) {
			throw new Error('the messenger account changed');
		}
		const key = JSON.stringify([scope.projectURL, scope.accountID, scope.companyID]);
		if (active && active.key !== key) {
			generation += 1;
			for (const reset of resets) reset();
		}
		const resolved = { key, generation };
		active = resolved;
		await requireCurrentMessengerScope(resolved);
		return resolved;
	});
	reading = attempt;
	try {
		return await attempt;
	} catch (error) {
		if (reading === attempt) reading = null;
		throw error;
	}
}

export function conversationStorageKey(scope: MessengerCacheScope): string {
	return storagePrefix + scope.key;
}
