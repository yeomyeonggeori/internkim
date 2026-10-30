import { callingAgent } from '../_shared/agent-caller.ts';
import { askedObject, refuse, serveRefusals } from '../_shared/http.ts';
import { costOfAnswer, creditsOfCost, streamCostReader } from '../_shared/model-call-cost.ts';
import type { SupabaseClient } from '../_shared/service-client.ts';

const openRouterChatURL = 'https://openrouter.ai/api/v1/chat/completions';
const defaultUsageLimit = 10_000;
const memberHeader = 'x-internkim-member';
const guidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type Charge = { client: SupabaseClient; companyID: string; memberID: string };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client, companyID } = await callingAgent(request);
		const memberID = request.headers.get(memberHeader) ?? '';
		if (!guidPattern.test(memberID)) refuse(400, `which member asked: their id in the ${memberHeader} header`);

		const { data: refusal, error } = await client.rpc('company_member_credit_refusal', {
			target_company: companyID,
			target_member: memberID,
			default_usage_limit: defaultUsageLimit
		});
		if (error) refuse(502, `the record could not say whether this member may call: ${error.message}`);
		if (refusal === 'not_a_member') refuse(403, 'that member is not one of this company');
		if (typeof refusal === 'string') refuse(402, refusal);

		const openRouterKey = Deno.env.get('OPENROUTER_API_KEY') ?? '';
		if (!openRouterKey) refuse(503, 'this deployment holds no OpenRouter key');

		const asked = await askedObject(request);
		const upstream = await fetch(openRouterChatURL, {
			method: 'POST',
			headers: { Authorization: `Bearer ${openRouterKey}`, 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...asked, usage: { include: true } })
		});
		if (!upstream.ok || !upstream.body) {
			return new Response(upstream.body, { status: upstream.status, headers: { 'Content-Type': 'application/json' } });
		}

		const charge: Charge = { client, companyID, memberID };
		if (asked.stream === true) return streamedAndCharged(upstream.body, charge);

		const answer: unknown = await upstream.json();
		await charged(charge, costOfAnswer(answer));
		return new Response(JSON.stringify(answer), { headers: { 'Content-Type': 'application/json' } });
	})
);

function streamedAndCharged(upstream: ReadableStream<Uint8Array>, charge: Charge): Response {
	const reader = streamCostReader();
	const decoder = new TextDecoder();
	const body = upstream.pipeThrough(
		new TransformStream<Uint8Array, Uint8Array>({
			transform(chunk, controller) {
				reader.read(decoder.decode(chunk, { stream: true }));
				controller.enqueue(chunk);
			},
			async flush() {
				reader.read(decoder.decode());
				await charged(charge, reader.cost());
			}
		})
	);
	return new Response(body, { headers: { 'Content-Type': 'text/event-stream' } });
}

async function charged(charge: Charge, cost: number | null): Promise<void> {
	if (cost === null) {
		console.error(JSON.stringify({ event: 'model_gateway.cost_missing', companyID: charge.companyID, memberID: charge.memberID }));
		return;
	}
	const { error } = await charge.client.rpc('company_credit_usage_charged', {
		target_company: charge.companyID,
		target_member: charge.memberID,
		used_credits: creditsOfCost(cost)
	});
	if (error) {
		console.error(
			JSON.stringify({ event: 'model_gateway.charge_failed', companyID: charge.companyID, memberID: charge.memberID, cost, reason: error.message })
		);
	}
}
