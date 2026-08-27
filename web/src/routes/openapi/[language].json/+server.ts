import { error, json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';
import { createOpenApiDocument } from '$lib/server/openapi';

export const GET: RequestHandler = ({ params }) => {
	if (params.language !== 'ko' && params.language !== 'en') error(404, 'OpenAPI document not found');
	return json(createOpenApiDocument(params.language, env.CLOUDFLARE_DOMAIN || undefined), {
		headers: { 'Cache-Control': 'public, max-age=0, s-maxage=3600' }
	});
};
