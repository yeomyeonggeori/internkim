import { supabase } from '$lib/supabase';

export type FoundingInvitation = {
	memberID: string;
	email: string;
	temporaryPassword: string;
};

export type FoundedCompany = {
	companyID: string;
	slug: string;
	invitations: FoundingInvitation[];
	address: { hostname: string | null; status: string };
};

export type AddressCheck = {
	slug: string;
	taken: boolean;
	usable: boolean;
	name: string | null;
};

export async function checkCompanyAddress(slug: string): Promise<AddressCheck> {
	const response = await fetch(`/api/company?slug=${encodeURIComponent(slug)}`);
	if (!response.ok) throw new Error((await response.text()).trim() || `the check returned ${response.status}`);
	return (await response.json()) as AddressCheck;
}

export async function foundCompany(company: {
	name: string;
	slug: string;
	invited: string[];
}): Promise<FoundedCompany> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/company', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(company)
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `founding returned ${response.status}`);
	return (await response.json()) as FoundedCompany;
}

export async function belongsToACompany(): Promise<boolean> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return false;
	const response = await fetch('/api/member/me', { headers: { Authorization: `Bearer ${accessToken}` } });
	if (!response.ok) return false;
	const claimed = (await response.json()) as { member: { memberID: string } | null };
	return claimed.member !== null;
}
