import { supabase } from '$lib/supabase';

export function isPasskeySupported(): boolean {
	return typeof window !== 'undefined' && typeof window.PublicKeyCredential !== 'undefined';
}

export async function registerPasskey(): Promise<void> {
	const { error } = await supabase().auth.registerPasskey();
	if (error) throw new Error(error.message);
}

export async function signInWithPasskey(): Promise<void> {
	const { error } = await supabase().auth.signInWithPasskey();
	if (error) throw new Error(error.message);
}
