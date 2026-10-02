import { redirect } from '@sveltejs/kit';
import { companyPathOf, companySlugOf } from '$lib/company-path';

export function load({ url }: { url: URL }) {
	redirect(308, companyPathOf(companySlugOf(url.pathname), '/files/data-room/'));
}
