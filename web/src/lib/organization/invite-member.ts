import { supabase } from '$lib/supabase';

export type MemberInvitation = {
	memberID: string;
	email: string;
	temporaryPassword: string;
};

export async function inviteMemberToCompany(email: string): Promise<MemberInvitation> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/invite', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `invite returned ${response.status}`);
	return (await response.json()) as MemberInvitation;
}
