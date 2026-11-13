import { redirect } from '@sveltejs/kit';
import { homePath } from '$lib/home-path';
import { isHydratingDocument } from '$lib/hydrated-document';
import type { PageLoad } from './$types';

const isBoard = import.meta.env.VITE_BUILD_TARGET === 'board';

export const load: PageLoad = async ({ parent }) => {
	if (isBoard) redirect(307, homePath);
	const { session } = await parent();
	if (!session?.authenticated) return { signedInHome: null };
	if (!isHydratingDocument()) redirect(307, homePath);
	return { signedInHome: homePath };
};
