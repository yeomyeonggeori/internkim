import { clearCachedAttendanceSummaries } from '../routes/attendance/attendance-summary-cache';
import { clearCachedWorkStatusRows } from '../routes/attendance/work-status/work-status-cache';
import { forgetKeptPictures } from '$lib/stores/person-picture-cache';
import { clearChannelMessageCache } from '$lib/components/channel/channel-message-cache';
import type { SupabaseClient } from '@supabase/supabase-js';
import { isSupabaseConfigured, supabase } from '$lib/supabase';
import { memberRoleOf, type MemberRole } from '$lib/member-vocabulary';
import { forgetLastSeenTask } from '../routes/task/task-last-seen';
import { forgetLastSeenDirectory } from '../routes/organization/organization-last-seen';
import { signedOutSession, type WebAuthSession } from '$lib/web-auth-session';
import { stopBeingReached } from '$lib/notifications/subscribe';
import { forgetSignedInAccount, signedInMember } from '$lib/signed-in-account-memo';
import { forgetWidgetSupply } from '$lib/widget/attendance-widget-supply';
import { releaseActivityTokens } from '$lib/widget/attendance-activity-tokens';

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

export type SignedInMember = {
	memberID: string;
	companyID: string;
	role: MemberRole;
	name: string;
	companySlug: string;
	companyLocale: string;
};

export async function supabaseMember(): Promise<SignedInMember> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) {
		return { memberID: '', companyID: '', role: 'member', name: '', companySlug: '', companyLocale: '' };
	}
	return signedInMember.of(accountID, async () => {
		const member = await supabase()
			.from('member')
			.select('id, company_id, is_admin, name, company (slug, locale)')
			.eq('user_id', accountID)
			.maybeSingle<{
				id: string;
				company_id: string;
				is_admin: boolean;
				name: string | null;
				company: { slug: string; locale: string } | null;
			}>();
		return {
			memberID: member.data?.id ?? '',
			companyID: member.data?.company_id ?? '',
			role: memberRoleOf(member.data?.is_admin ?? false),
			name: member.data?.name ?? '',
			companySlug: member.data?.company?.slug ?? '',
			companyLocale: member.data?.company?.locale ?? ''
		};
	});
}

export async function supabaseMemberRole(): Promise<MemberRole> {
	return (await supabaseMember()).role;
}

export async function signOutOfSupabase(): Promise<void> {
	const { forgetCentralBuzzIdentity } = await import('$lib/central-buzz-identity');
	forgetCentralBuzzIdentity();
	await stopBeingReached().catch(() => undefined);
	await forgetWidgetSupply().catch(() => undefined);
	await releaseActivityTokens().catch(() => undefined);
	clearCachedAttendanceSummaries();
	clearCachedWorkStatusRows();
	forgetKeptPictures();
	clearChannelMessageCache();
	forgetLastSeenTask();
	forgetLastSeenDirectory();
	forgetSignedInAccount(true);
	await supabase().auth.signOut({ scope: 'local' });
}

export type ClaimOutcome =
	| { kind: 'sent' }
	| { kind: 'tooManyLately' }
	| { kind: 'failed' };

export function askToClaim(email: string): Promise<ClaimOutcome> {
	return askForCode('/api/auth/claim', email);
}

export function askToStartCompany(email: string): Promise<ClaimOutcome> {
	return askForCode('/api/auth/signup', email);
}

async function askForCode(path: string, email: string): Promise<ClaimOutcome> {
	const response = await fetch(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email: email.trim().toLowerCase() })
	});
	if (response.status === 429) return { kind: 'tooManyLately' };
	if (!response.ok) return { kind: 'failed' };
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
