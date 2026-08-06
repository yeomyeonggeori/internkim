import { docs, versions } from 'svelte-docsmith/content';
import { generateSitemap, currentOnly } from 'svelte-docsmith';
import { siteConfig } from '$lib/site-config';

export const prerender = true;

export function GET() {
	const body = generateSitemap(siteConfig.url ?? '', [
		{ path: '/' },
		...currentOnly(docs, versions).map((page) => ({ path: page.path, lastmod: page.lastUpdated }))
	]);
	return new Response(body, { headers: { 'content-type': 'application/xml' } });
}
