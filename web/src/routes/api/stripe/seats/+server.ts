import { environmentOf } from '$lib/server/agent-request';
import { callingScheduledJob } from '$lib/server/scheduled-job-request';
import { followSeats } from '$lib/server/stripe-seats';
import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { RequestHandler } from './$types';

const seatsAsk = z.object({ company_id: z.guid() });

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const secretKey = environment.STRIPE_SECRET_KEY ?? '';
	if (!secretKey) error(503, 'Stripe billing is not configured');

	const client = await callingScheduledJob(request, environment);
	const ask = seatsAsk.safeParse(await request.json().catch(() => null));
	if (!ask.success) error(400, `the seats ask names no company: ${z.prettifyError(ask.error)}`);

	return json({ outcome: await followSeats(client, secretKey, ask.data.company_id) });
};
