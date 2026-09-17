import { isHttpError, json } from '@sveltejs/kit';

export const protectedResourceMetadataPath = '/.well-known/oauth-protected-resource';

const toolServerPaths = new Set(['/v1/mcp', '/api/v1/mcp']);

export type ProtectedResourceMetadata = {
	resource: string;
	authorization_servers: string[];
	bearer_methods_supported: string[];
	resource_name: string;
	resource_documentation: string;
};

export function isToolServerPath(pathname: string): boolean {
	return toolServerPaths.has(pathname);
}

export function authorizationServerOf(projectURL: string): string {
	return `${projectURL.replace(/\/+$/, '')}/auth/v1`;
}

export function protectedResourceMetadataOf(resource: URL, projectURL: string): ProtectedResourceMetadata {
	return {
		resource: `${resource.origin}${resource.pathname}`,
		authorization_servers: [authorizationServerOf(projectURL)],
		bearer_methods_supported: ['header'],
		resource_name: 'InternKim',
		resource_documentation: 'https://docs.intern.kim/docs/api'
	};
}

function authorizationChallengeTo(resource: URL, message: string): Response {
	const metadataAddress = `${resource.origin}${protectedResourceMetadataPath}${resource.pathname}`;
	return json(
		{ message },
		{ status: 401, headers: { 'WWW-Authenticate': `Bearer resource_metadata="${metadataAddress}"` } }
	);
}

export function challengingTheUnauthenticated(resource: URL): (refusal: unknown) => Response {
	return (refusal) => {
		if (isHttpError(refusal, 401)) return authorizationChallengeTo(resource, refusal.body.message);
		throw refusal;
	};
}
