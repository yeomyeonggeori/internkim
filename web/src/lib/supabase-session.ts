import type { SupabaseClient } from '@supabase/supabase-js';
import { isSupabaseConfigured, supabase } from '$lib/supabase';
import { forgetHeldTasks } from '$lib/task/task-cache';
import { forgetLastSeenTask } from '../routes/task/task-last-seen';
import { forgetLastSeenDirectory } from '../routes/organization/organization-last-seen';
import { signedOutSession, type WebAuthSession } from '$lib/web-auth-session';
import { stopBeingReached } from '$lib/notifications/subscribe';

export { isSupabaseConfigured };

export async function supabaseWebAuthSession(returnPath: string): Promise<WebAuthSession> {
	const { data } = await supabase().auth.getSession();
	const email = data.session?.user.email;
	if (!email) return signedOutSession(returnPath, false);
	return {
		authenticated: true,
		email,
		image: '',
		canViewTasks: true,
		cloudflareLoginURL: '',
		isUnavailable: false
	};
}

export type SignedInMember = { memberID: string; role: 'admin' | 'member'; companySlug: string };

export async function supabaseMember(): Promise<SignedInMember> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return { memberID: '', role: 'member', companySlug: '' };
	const member = await supabase()
		.from('member')
		.select('id, is_admin, company (slug)')
		.eq('user_id', accountID)
		.maybeSingle<{ id: string; is_admin: boolean; company: { slug: string } | null }>();
	return {
		memberID: member.data?.id ?? '',
		role: member.data?.is_admin ? 'admin' : 'member',
		companySlug: member.data?.company?.slug ?? ''
	};
}

export async function supabaseMemberRole(): Promise<'admin' | 'member'> {
	return (await supabaseMember()).role;
}

export async function signOutOfSupabase(): Promise<void> {
	await stopBeingReached().catch(() => undefined);
	forgetHeldTasks();
	forgetLastSeenTask();
	forgetLastSeenDirectory();
	await supabase().auth.signOut({ scope: 'local' });
}

export type ClaimOutcome =
	| { kind: 'sent' }
	| { kind: 'issued'; password: string }
	| { kind: 'alreadyClaimed' }
	| { kind: 'tooManyLately' }
	| { kind: 'failed' };

export async function askToClaim(email: string): Promise<ClaimOutcome> {
	const response = await fetch('/api/auth/claim', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email: email.trim().toLowerCase() })
	});
	if (response.status === 429) return { kind: 'tooManyLately' };
	if (response.status === 409) return { kind: 'alreadyClaimed' };
	if (!response.ok) return { kind: 'failed' };

	const answered = (await response.json().catch(() => ({}))) as { password?: unknown };
	if (typeof answered.password === 'string' && answered.password) {
		return { kind: 'issued', password: answered.password };
	}
	return { kind: 'sent' };
}

export async function verifyClaimCode(email: string, code: string): Promise<void> {
	const { error } = await supabase().auth.verifyOtp({
		email: email.trim().toLowerCase(),
		token: code.trim(),
		type: 'email'
	});
	if (error) throw new Error(error.message);
}

export class WrongPasswordError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'WrongPasswordError';
	}
}

export async function setSupabasePassword(
	password: string,
	client: SupabaseClient = supabase()
): Promise<void> {
	const { error } = await client.auth.updateUser({ password });
	if (error) throw new Error(error.message);
}

export async function signInWithSupabase(
	email: string,
	password: string,
	client: SupabaseClient = supabase()
): Promise<void> {
	const { error } = await client.auth.signInWithPassword({
		email: email.trim().toLowerCase(),
		password
	});
	if (!error) return;
	if (error.code === 'invalid_credentials') throw new WrongPasswordError(error.message);
	throw new Error(error.message);
}

export async function changeOwnPassword(
	currentPassword: string,
	newPassword: string,
	client: SupabaseClient = supabase()
): Promise<void> {
	const { data } = await client.auth.getSession();
	const email = data.session?.user.email;
	if (!email) throw new Error('no signed-in account to change a password for');

	await signInWithSupabase(email, currentPassword, client);
	await setSupabasePassword(newPassword, client);
}
