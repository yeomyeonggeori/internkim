import { supabase } from '$lib/supabase';
import { callCompanyApp } from '$lib/host-bridge';

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
	const invitation = (await response.json()) as MemberInvitation;
	await tellTheCompanyServerTheDirectoryChanged();
	return invitation;
}

// The company records the member and the company's own server gives them a
// Linux user, so it is told rather than left to notice on its next sweep.
// Whoever was just invited would otherwise be refused as a stranger until then,
// and a server that cannot be reached is not a failed invitation.
export async function tellTheCompanyServerTheDirectoryChanged(): Promise<void> {
	try {
		await callCompanyApp({ capability: 'directory.changed' });
	} catch {
		return;
	}
}
