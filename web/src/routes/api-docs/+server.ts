import { redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { preferredDocumentationLanguage } from '$lib/server/api-documentation-language';

export const GET: RequestHandler = ({ request }) => {
	const language = preferredDocumentationLanguage(request.headers.get('accept-language'));
	redirect(307, `/api-docs/${language}`);
};
