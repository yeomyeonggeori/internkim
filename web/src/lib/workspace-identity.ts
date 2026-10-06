import type { SignedInMember } from '$lib/supabase-session';
import type { WebAuthSession } from '$lib/web-auth-session';

export function workspaceIdentityKey(
	session: WebAuthSession | null,
	member?: SignedInMember,
	project = ''
): string {
	return JSON.stringify([
		project, session?.authenticated ?? false, session?.email ?? '',
		member?.companyID ?? '', member?.memberID ?? '', member?.role ?? ''
	]);
}
