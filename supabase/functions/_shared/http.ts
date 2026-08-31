export function json(body: Record<string, unknown>, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

export function refuse(status: number, message: string): never {
	throw json({ error: message }, status);
}

export function serveRefusals(
	handle: (request: Request) => Promise<Response>
): (request: Request) => Promise<Response> {
	return async (request) => {
		try {
			return await handle(request);
		} catch (thrown) {
			if (thrown instanceof Response) return thrown;
			throw thrown;
		}
	};
}
