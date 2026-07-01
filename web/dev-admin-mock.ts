import type { IncomingMessage, ServerResponse } from 'node:http';

export type DevAdminMockState = {
	userEmail: string;
	userRole: DevAdminMockUserRole;
	locale: 'ko' | 'en';
};

export type DevAdminMockUserRole = 'admin' | 'operationsAdmin' | 'member';

export type DevMockRequest = {
	method: string;
	pathname: string;
	searchParams: URLSearchParams;
	body?: string;
};

export type DevMockResponse = {
	status: number;
	body: unknown;
};

export function createDevAdminMockState(userEmail: string, userRole: DevAdminMockUserRole = 'admin'): DevAdminMockState {
	return {
		userEmail,
		userRole,
		locale: 'ko'
	};
}

export function createDevAdminMockResponse(
	state: DevAdminMockState,
	request: DevMockRequest
): DevMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/auth/session') {
		return { status: 200, body: { authenticated: true, email: state.userEmail, isAdmin: state.userRole === 'admin' } };
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/session') {
		if (state.userRole === 'member') {
			return { status: 403, body: { error: 'admin access required' } };
		}
		return {
			status: 200,
			body: {
				email: state.userEmail,
				claimedAdminEmail: state.userEmail,
				isAdmin: state.userRole === 'admin',
				role: state.userRole,
				isClaimed: true,
				bootstrapStatus: 'claimed'
			}
		};
	}
	if (request.method === 'GET' && request.pathname === '/admin/api/locale') {
		return { status: 200, body: { locale: state.locale } };
	}
	if (request.method === 'PUT' && request.pathname === '/admin/api/locale') {
		state.locale = localeFromBody(request.body);
		return { status: 200, body: { locale: state.locale } };
	}
	return undefined;
}

export function shouldHandleDevAdminMockRequest(method: string, pathname: string): boolean {
	if (method === 'GET' && pathname === '/auth/session') return true;
	if (method === 'GET' && pathname === '/admin/api/session') return true;
	if (method === 'GET' && pathname === '/admin/api/locale') return true;
	return method === 'PUT' && pathname === '/admin/api/locale';
}

export function parseJSONRecord(body: string | undefined): Record<string, unknown> {
	if (!body) return {};
	try {
		const parsed: unknown = JSON.parse(body);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
	} catch {
		return {};
	}
	return {};
}

export function readRequestBody(request: IncomingMessage, callback: (body: string) => void): void {
	if (request.method === 'GET') {
		callback('');
		return;
	}
	let body = '';
	request.on('data', (chunk: Buffer) => {
		body += chunk.toString('utf8');
	});
	request.on('end', () => callback(body));
	request.on('error', () => callback(''));
}

export function writeJSON(response: ServerResponse, status: number, body: unknown): void {
	response.statusCode = status;
	response.setHeader('Content-Type', 'application/json');
	response.end(JSON.stringify(body));
}

function localeFromBody(body: string | undefined): 'ko' | 'en' {
	const parsed = parseJSONRecord(body);
	return parsed.locale === 'en' ? 'en' : 'ko';
}
