import { environmentOf } from '$lib/server/agent-request';
import {
	isToolServerPath,
	protectedResourceMetadataOf,
	protectedResourceMetadataPath
} from '$lib/server/public-api/protected-resource';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = ({ url, platform }) => {
	const resource = new URL(url.pathname.slice(protectedResourceMetadataPath.length), url.origin);
	if (!isToolServerPath(resource.pathname)) error(404, 'no protected resource lives there');

	const projectURL = environmentOf(platform).SUPABASE_URL ?? '';
	if (!projectURL) error(500, 'the control plane is not configured');

	return json(protectedResourceMetadataOf(resource, projectURL));
};
