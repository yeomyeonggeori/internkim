import { environmentOf } from '$lib/server/agent-request';
import { callingScheduledJob } from '$lib/server/scheduled-job-request';
import { chargeTopUp } from '$lib/server/stripe-top-up';
import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { RequestHandler } from './$types';

const topUpAsk = z.object({ top_up_id: z.guid() });

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const secretKey = environment.STRIPE_SECRET_KEY ?? '';
	if (!secretKey) error(503, 'Stripe billing is not configured');

	const client = await callingScheduledJob(request, environment);
	const ask = topUpAsk.safeParse(await request.json().catch(() => null));
	if (!ask.success) error(400, `the top-up ask names no top-up: ${z.prettifyError(ask.error)}`);

	return json({ outcome: await chargeTopUp(client, secretKey, ask.data.top_up_id) });
};
