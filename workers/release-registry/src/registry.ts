import type { ReleaseRegistryEnvironment } from './index';

const releaseTokenHeader = 'X-InternKim-Release-Token';

export async function handleReleaseRegistryRequest(request: Request, environment: ReleaseRegistryEnvironment): Promise<Response> {
	if (!isAllowedMethod(request.method)) {
		return textResponse('method not allowed', 405);
	}
	if (!isAuthorized(request, environment.RELEASE_DOWNLOAD_TOKEN)) {
		return textResponse('unauthorized', 401);
	}
	const objectKey = objectKeyFromRequest(request);
	if (!objectKey) {
		return textResponse('not found', 404);
	}
	const object = await environment.RELEASE_BUCKET.get(objectKey);
	if (!object) {
		return textResponse('not found', 404);
	}
	return objectResponse(request, object);
}

function isAllowedMethod(method: string): boolean {
	return method === 'GET' || method === 'HEAD';
}

function isAuthorized(request: Request, expectedToken?: string): boolean {
	const token = expectedToken?.trim();
	if (!token) {
		return false;
	}
	return request.headers.get(releaseTokenHeader) === token;
}

function objectKeyFromRequest(request: Request): string {
	const url = new URL(request.url);
	const rawURL = request.url.toLowerCase();
	if (rawURL.includes('%2e') || rawURL.includes('%5c')) {
		return '';
	}
	const key = decodeURIComponent(url.pathname).replace(/^\/+/, '');
	if (!key || key.includes('..') || key.includes('\\') || key.includes('%')) {
		return '';
	}
	return key;
}

function objectResponse(request: Request, object: R2ObjectBody): Response {
	const headers = new Headers();
	object.writeHttpMetadata(headers);
	headers.set('etag', object.httpEtag);
	if (object.size !== undefined) {
		headers.set('content-length', String(object.size));
	}
	if (request.method === 'HEAD') {
		return new Response(null, { status: 200, headers });
	}
	return new Response(object.body, { status: 200, headers });
}

function textResponse(text: string, status: number): Response {
	return new Response(text + '\n', { status, headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
}
