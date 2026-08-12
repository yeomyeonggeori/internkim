import { redirect } from '@sveltejs/kit';
import { belongsToACompany } from '$lib/company/found-company';
import { isEmbeddedFrame } from '$lib/embedded';
import { isSupabaseConfigured, supabaseMember, supabaseWebAuthSession } from '$lib/supabase-session';
import { companyPathOf, wantsCompanyPrefix } from '$lib/company-path';
import { signedOutSession, webAuthSessionDependency, webAuthSessionFrom, type WebAuthSession } from '$lib/web-auth-session';
import type { LayoutLoad } from './$types';

const isBoard = import.meta.env.VITE_BUILD_TARGET === 'board';

export const prerender = isBoard;
export const ssr = !isBoard;

export const load: LayoutLoad<{ session: WebAuthSession | null }> = async ({ fetch, depends }) => {
	depends(webAuthSessionDependency);
	if (typeof window === 'undefined' || isEmbeddedFrame()) return { session: null };
	const returnPath = window.location.pathname + window.location.search;
	if (isSupabaseConfigured()) {
		const session = await supabaseWebAuthSession(returnPath);
		const settlingIn = returnPath.startsWith('/start') || returnPath.startsWith('/auth/');
		if (session.authenticated && !settlingIn && !(await belongsToACompany())) {
			redirect(307, '/start');
		}
		if (session.authenticated && wantsCompanyPrefix(window.location.pathname)) {
			const { companySlug } = await supabaseMember();
			if (companySlug) {
				redirect(307, companyPathOf(companySlug, window.location.pathname) + window.location.search);
			}
		}
		return { session };
	}
	try {
		const response = await fetch(`/auth/session?return=${encodeURIComponent(returnPath)}`, { credentials: 'include' });
		if (!response.ok) throw new Error(`session returned ${response.status}`);
		return { session: webAuthSessionFrom(await response.json(), returnPath) };
	} catch {
		return { session: signedOutSession(returnPath, true) };
	}
};
