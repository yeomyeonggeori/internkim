import { z } from 'zod';
import { supabase } from '$lib/supabase';

export class SharedTasksRefused extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
	}
}

export async function signedInEmail(): Promise<string | null> {
	const { data } = await supabase().auth.getSession();
	return data.session?.user.email ?? null;
}

export async function sharedTasksRequest(path: string, method = 'GET'): Promise<unknown> {
	const { data } = await supabase().auth.getSession();
	if (!data.session) throw new SharedTasksRefused('sign in first', 401);
	const response = await fetch(path, { method, headers: { Authorization: `Bearer ${data.session.access_token}` } });
	const answer: unknown = await response.json().catch(() => null);
	if (response.ok) return answer;
	const refusal = z.object({ message: z.string() }).safeParse(answer);
	throw new SharedTasksRefused(refusal.success ? refusal.data.message : `shared tasks returned ${response.status}`, response.status);
}

export async function requestGuestCode(email: string): Promise<void> {
	const { error } = await supabase().auth.signInWithOtp({ email, options: { shouldCreateUser: false } });
	if (error) throw error;
}

export async function verifyGuestCode(email: string, code: string): Promise<void> {
	const { error } = await supabase().auth.verifyOtp({ email, token: code, type: 'email' });
	if (error) throw error;
}
