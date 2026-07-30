import { isEmbeddedFrame } from '$lib/embedded';
import { signedOutSession, webAuthSessionDependency, webAuthSessionFrom, type WebAuthSession } from '$lib/web-auth-session';
import type { LayoutLoad } from './$types';

const isBoard = import.meta.env.VITE_BUILD_TARGET === 'board';

export const prerender = isBoard;
export const ssr = !isBoard;

export const load: LayoutLoad<{ session: WebAuthSession | null }> = async ({ fetch, depends }) => {
	depends(webAuthSessionDependency);
	if (typeof window === 'undefined' || isEmbeddedFrame()) return { session: null };
	const returnPath = window.location.pathname + window.location.search;
	try {
		const response = await fetch(`/auth/session?return=${encodeURIComponent(returnPath)}`, { credentials: 'include' });
		if (!response.ok) throw new Error(`session returned ${response.status}`);
		return { session: webAuthSessionFrom(await response.json(), returnPath) };
	} catch {
		return { session: signedOutSession(returnPath, true) };
	}
};
