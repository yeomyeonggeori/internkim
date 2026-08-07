import { docs } from 'svelte-docsmith/llms';
import { error } from '@sveltejs/kit';
import type { EntryGenerator, RequestHandler } from './$types';

export const prerender = true;

export const entries: EntryGenerator = () =>
	docs.map((page) => ({ slug: page.path.replace(/^\//, '') }));

export const GET: RequestHandler = ({ params }) => {
	const requested = docs.find((page) => page.path === `/${params.slug}`);
	if (!requested) error(404, 'Not found');
	return new Response(requested.content, {
		headers: { 'content-type': 'text/markdown; charset=utf-8' }
	});
};
