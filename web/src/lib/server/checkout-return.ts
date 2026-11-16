import { error } from '@sveltejs/kit';
import { z } from 'zod';

const checkoutRequest = z.object({ returnPath: z.string().optional() });

export async function checkoutReturnURL(request: Request, origin: string): Promise<string> {
	const body = checkoutRequest.safeParse(await request.json().catch(() => ({})));
	if (!body.success) error(400, 'the checkout request must be an object whose optional returnPath is a string');
	const requested = body.data.returnPath;
	if (!requested) return origin;
	const resolved = new URL(requested, origin);
	if (!requested.startsWith('/') || resolved.origin !== origin) {
		error(400, `returnPath ${requested} leaves ${origin}; pass a path on this site such as /settings/setup`);
	}
	return `${origin}${resolved.pathname}`;
}
