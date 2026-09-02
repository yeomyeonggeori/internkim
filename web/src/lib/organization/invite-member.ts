import { supabase } from '$lib/supabase';
import { callCompanyApp } from '$lib/host-bridge';

export type MemberInvitation = {
	memberID: string;
	email: string;
	temporaryPassword: string;
};

export async function inviteMemberToCompany(email: string, name: string): Promise<MemberInvitation> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/invite', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, name })
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

// A member removed here is a former colleague everywhere: the company server is
// told so it can take back their seats in the rooms and their place in the
// community, rather than waiting for the pass that runs once a day.
export async function resetMemberPassword(memberID: string): Promise<MemberInvitation> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/password-reset', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ memberID })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `password reset returned ${response.status}`);
	return (await response.json()) as MemberInvitation;
}

export type WipedMemberMessages = {
	email: string;
	buzzDeleted: number;
	mattermostDeleted: number;
};

export async function wipeMemberMessages(email: string): Promise<WipedMemberMessages> {
	const response = await fetch('/agent/api/buzz-admin-wipe', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `message wipe returned ${response.status}`);
	return (await response.json()) as WipedMemberMessages;
}

export async function removeMemberFromCompany(memberID: string, purge = false): Promise<{ wasRemoved: boolean }> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/remove', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ memberID, purge })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `remove returned ${response.status}`);
	const removal = (await response.json()) as { wasRemoved: boolean };
	await tellTheCompanyServerTheDirectoryChanged();
	return removal;
}
