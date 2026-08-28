import { redirect } from '@sveltejs/kit';

export function load({ url }): never {
	redirect(308, '/task' + url.search);
}
