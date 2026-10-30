export type ModelGatewayRoute = {
	upstreamPath: string;
	isCharged: boolean;
};

const routes: Record<string, ModelGatewayRoute> = {
	'POST /api/v1/chat/completions': { upstreamPath: '/api/v1/chat/completions', isCharged: true },
	'POST /api/alpha/decisions': { upstreamPath: '/api/alpha/decisions', isCharged: true },
	'GET /api/v1/models': { upstreamPath: '/api/v1/models', isCharged: false }
};

export function modelGatewayRoute(method: string, pathname: string): ModelGatewayRoute | null {
	const apiStart = pathname.indexOf('/api/');
	if (apiStart < 0) return null;
	return routes[`${method} ${pathname.slice(apiStart).replace(/\/+$/, '')}`] ?? null;
}
