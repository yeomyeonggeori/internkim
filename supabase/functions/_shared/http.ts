const crossOriginHeaders: Record<string, string> = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Headers': 'authorization, content-type, apikey, x-client-info',
	'Access-Control-Allow-Methods': 'POST, OPTIONS'
};

export function json(body: Record<string, unknown>, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json', ...crossOriginHeaders }
	});
}

export function refuse(status: number, message: string): never {
	throw json({ error: message }, status);
}

export async function askedObject(request: Request): Promise<Record<string, unknown>> {
	const parsed = await request.json().catch(() => null);
	if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {};
	return parsed as Record<string, unknown>;
}

export function serveRefusals(
	handle: (request: Request) => Promise<Response>
): (request: Request) => Promise<Response> {
	return async (request) => {
		if (request.method === 'OPTIONS') {
			return new Response(null, { status: 204, headers: crossOriginHeaders });
		}
		try {
			return await handle(request);
		} catch (thrown) {
			if (thrown instanceof Response) return thrown;
			throw thrown;
		}
	};
}
