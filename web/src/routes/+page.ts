import { redirect } from '@sveltejs/kit';
import { homePath } from '$lib/home-path';

export function load() {
	redirect(307, homePath);
}
