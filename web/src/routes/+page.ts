import { redirect } from '@sveltejs/kit';
import { homePath } from '$lib/home-path';
import type { PageLoad } from './$types';

const isBoard = import.meta.env.VITE_BUILD_TARGET === 'board';

export const load: PageLoad = async ({ parent }) => {
	if (isBoard) redirect(307, homePath);
	const { session } = await parent();
	if (session?.authenticated) redirect(307, homePath);
};
