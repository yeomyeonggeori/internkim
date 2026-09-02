import type { SupabaseClient } from '@supabase/supabase-js';
import { supabase } from '$lib/supabase';

export class WrongCurrentPasswordError extends Error {
	constructor() {
		super('the current password does not match');
		this.name = 'WrongCurrentPasswordError';
	}
}

const badRequest = 400;

export async function changeOwnPassword(
	currentPassword: string,
	newPassword: string,
	client: SupabaseClient = supabase(),
): Promise<void> {
	const email = await signedInAddress(client);
	await proveCurrentPassword(client, email, currentPassword);

	const { error } = await client.auth.updateUser({ password: newPassword });
	if (error) throw new Error(error.message);
}

async function signedInAddress(client: SupabaseClient): Promise<string> {
	const { data } = await client.auth.getSession();
	const email = data.session?.user.email;
	if (!email) throw new Error('no signed-in account to change a password for');
	return email;
}

async function proveCurrentPassword(
	client: SupabaseClient,
	email: string,
	currentPassword: string,
): Promise<void> {
	const { error } = await client.auth.signInWithPassword({ email, password: currentPassword });
	if (!error) return;
	if (error.status === badRequest) throw new WrongCurrentPasswordError();
	throw new Error(error.message);
}
