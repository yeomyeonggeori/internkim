import { projectURL, supabase } from '$lib/supabase';

export type CompanySessionScope = {
	projectURL: string;
	accountID: string;
	companyID: string;
	sessionKey: string;
	accessToken: string;
};

export type StoredCompanyScope = Omit<CompanySessionScope, 'accessToken'>;

export function sameCompanySessionScope(first: StoredCompanyScope, second: StoredCompanyScope): boolean {
	return first.projectURL === second.projectURL && first.accountID === second.accountID &&
		first.companyID === second.companyID && first.sessionKey === second.sessionKey;
}

export async function readVerifiedCompanyScope(): Promise<CompanySessionScope | null> {
	const client = supabase();
	const project = projectURL();
	const { data: before } = await client.auth.getSession();
	const accessToken = before.session?.access_token;
	if (!accessToken) return null;
	const { data: account, error } = await client.auth.getUser(accessToken);
	if (error || !account.user) return null;
	const member = await client.from('member').select('company_id').eq('user_id', account.user.id)
		.maybeSingle<{ company_id: string }>();
	if (member.error || !member.data?.company_id) return null;
	const fingerprint = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(accessToken));
	const sessionKey = Array.from(new Uint8Array(fingerprint), (byte) => byte.toString(16).padStart(2, '0')).join('');
	const { data: after } = await client.auth.getSession();
	if (after.session?.access_token !== accessToken || projectURL() !== project) return null;
	return { projectURL: project, accountID: account.user.id, companyID: member.data.company_id, sessionKey, accessToken };
}
