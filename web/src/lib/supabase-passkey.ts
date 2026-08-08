import { supabase } from '$lib/supabase';

export type Passkey = {
	id: string;
	name: string;
	createdAt: string;
	lastUsedAt: string | null;
};

export type PasskeyRefusal = 'cancelled' | 'already-registered' | 'failed';

const refusalByCode: Record<string, PasskeyRefusal> = {
	ERROR_CEREMONY_ABORTED: 'cancelled',
	ERROR_AUTHENTICATOR_PREVIOUSLY_REGISTERED: 'already-registered'
};

export function refusalOf(error: unknown): PasskeyRefusal {
	if (typeof error !== 'object' || error === null) return 'failed';
	const { code } = error as { code?: unknown };
	if (typeof code !== 'string') return 'failed';
	return refusalByCode[code] ?? 'failed';
}

export function isPasskeySupported(): boolean {
	return typeof window !== 'undefined' && typeof window.PublicKeyCredential !== 'undefined';
}

export async function registerPasskey(): Promise<void> {
	const { error } = await supabase().auth.registerPasskey();
	if (error) throw error;
}

export async function signInWithPasskey(): Promise<void> {
	const { error } = await supabase().auth.signInWithPasskey();
	if (error) throw error;
}

export async function listPasskeys(): Promise<Passkey[]> {
	const { data, error } = await supabase().auth.passkey.list();
	if (error) throw error;
	return (data ?? []).map((entry) => ({
		id: entry.id,
		name: entry.friendly_name?.trim() ?? '',
		createdAt: entry.created_at,
		lastUsedAt: entry.last_used_at ?? null
	}));
}

export async function forgetPasskey(passkeyID: string): Promise<void> {
	const { error } = await supabase().auth.passkey.delete({ passkeyId: passkeyID });
	if (error) throw error;
}
