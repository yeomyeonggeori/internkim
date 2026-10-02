import { mock } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const stored = new Map<string, string>();
Object.defineProperty(globalThis, 'sessionStorage', { value: {
	getItem: (key: string) => stored.get(key) ?? null,
	setItem: (key: string, value: string) => { stored.set(key, value); },
	removeItem: (key: string) => { stored.delete(key); }
} });

let token = 'sample-token-a';
let accountID = 'account-a';
let companyID = 'company-a';
let authorized = true;
let getUserCalls = 0;
let memberCalls = 0;
let claims = 0;
let onAuth: (event: string, session: { access_token: string } | null) => void = () => {};
let claim: () => Promise<string | null> = async () => 'same-derived-secret';
const owners: unknown[] = [];

mock.module('../../src/lib/supabase', () => ({
	projectURL: () => 'https://central.example.com',
	isSupabaseConfigured: () => true,
	supabase: () => ({
		auth: {
			getSession: async () => ({ data: { session: token ? { access_token: token, user: { id: accountID } } : null } }),
			getUser: async () => {
				getUserCalls += 1;
				return { data: { user: authorized ? { id: accountID } : null }, error: authorized ? null : new Error('revoked') };
			},
			onAuthStateChange: (callback: typeof onAuth) => {
				onAuth = callback;
				callback('INITIAL_SESSION', { access_token: token });
				return { data: { subscription: { unsubscribe() {} } } };
			}
		},
		from: () => ({ select: () => ({ eq: () => ({ maybeSingle: async () => {
			memberCalls += 1;
			return { data: { company_id: companyID }, error: null };
		} }) }) })
	})
}));
mock.module('../../src/lib/buzz-identity-central-login', () => ({
	claimCentralBuzzSecret: async (owner: unknown) => {
		claims += 1;
		owners.push(owner);
		return claim();
	}
}));

stored.set('internkim.buzz.secret', 'unscoped-old-account-key');
const { buzzIdentity } = await import('../../src/lib/stores/buzz-identity.svelte');
const { ensureCentralBuzzIdentity, invalidateCentralBuzzIdentity, watchCentralBuzzIdentity } = await import('../../src/lib/central-buzz-identity');
const initial = buzzIdentity.secretHex;
const stop = watchCentralBuzzIdentity();
const first = await ensureCentralBuzzIdentity();
const savedScope = JSON.parse(stored.get('internkim.buzz.scope') ?? 'null');
const storedToken = [...stored.values()].some((value) => value.includes('sample-token'));
invalidateCentralBuzzIdentity();
const hiddenWhileChecking = buzzIdentity.secretHex;
const restored = await ensureCentralBuzzIdentity();
const restoreClaims = claims;

companyID = 'company-b';
invalidateCentralBuzzIdentity();
claim = async () => 'company-b-key';
const changedCompany = await ensureCentralBuzzIdentity();

let resolveClaim: (secret: string) => void = () => {};
claim = () => new Promise((resolve) => { resolveClaim = resolve; });
token = 'sample-token-b';
onAuth('TOKEN_REFRESHED', { access_token: token });
const pending = ensureCentralBuzzIdentity();
while (claims < 3) await new Promise((resolve) => setTimeout(resolve, 0));
accountID = 'account-b';
token = '';
onAuth('SIGNED_OUT', null);
resolveClaim('late-account-a-key');
const late = await pending;
const afterSignout = buzzIdentity.secretHex;
const storedAfterSignout = stored.has('internkim.buzz.secret');

token = 'sample-token-c';
authorized = false;
const refused = await ensureCentralBuzzIdentity();
const finalClaims = claims;
stop();
console.log(JSON.stringify({ initial, first, savedScope, storedToken, hiddenWhileChecking, restored,
	restoreClaims, changedCompany, late, afterSignout, storedAfterSignout, refused, finalClaims,
	getUserCalls, memberCalls, owners }));
