import { deriveBuzzPasskeyOutput, loginWithBuzzPasskey, registerBuzzPasskey } from '$lib/buzz-passkey';
import { supabase } from '$lib/supabase';

function secretFrom(output: Uint8Array): string {
	let binary = '';
	for (const byte of output) binary += String.fromCharCode(byte);
	return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export async function enrolPasskey(email: string, displayName: string): Promise<void> {
	await registerBuzzPasskey(email, displayName);
	const { error } = await supabase().auth.updateUser({ password: secretFrom(await deriveBuzzPasskeyOutput()) });
	if (error) throw new Error(error.message);
}

export async function signInWithPasskey(): Promise<void> {
	const { email, output } = await loginWithBuzzPasskey();
	const { error } = await supabase().auth.signInWithPassword({ email, password: secretFrom(output) });
	if (error) throw new Error(error.message);
}
