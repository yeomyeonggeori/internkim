import { callingAgent } from '../_shared/agent-caller.ts';
import { defaultCreditUsageLimit } from '../_shared/credit-policy.ts';
import { askedObject, refuse, serveRefusals } from '../_shared/http.ts';
import { costOfAnswer, creditsOfCost, dollarsPerCreditOf, streamCostReader } from '../_shared/model-call-cost.ts';
import { membersOfCompanyByEmail } from '../_shared/member-directory.ts';
import { modelGatewayRoute } from '../_shared/model-gateway-route.ts';
import type { SupabaseClient } from '../_shared/service-client.ts';

const openRouterOrigin = 'https://openrouter.ai';
const requesterEmailHeader = 'x-internkim-requester-email';

type Charge = { client: SupabaseClient; companyID: string; memberID: string | null; dollarsPerCredit: number };

Deno.serve(
	serveRefusals(async (request) => {
		const route = modelGatewayRoute(request.method, new URL(request.url).pathname);
		if (!route) refuse(404, 'the model gateway answers chat completions, decisions and the model list');
		const { client, companyID } = await callingAgent(request);

		const openRouterKey = Deno.env.get('OPENROUTER_API_KEY') ?? '';
		if (!openRouterKey) refuse(402, 'this deployment holds no OpenRouter key, so it sells no model calls');
		const upstreamURL = openRouterOrigin + route.upstreamPath;
		const authorization = { Authorization: `Bearer ${openRouterKey}` };

		if (!route.isCharged) {
			const listed = await fetch(upstreamURL, { headers: authorization });
			return new Response(listed.body, { status: listed.status, headers: { 'Content-Type': 'application/json' } });
		}

		const dollarsPerCredit = dollarsPerCreditOf(Deno.env.get('MODEL_CREDIT_DOLLARS'));
		if (dollarsPerCredit === null) refuse(402, 'this deployment prices no credit in MODEL_CREDIT_DOLLARS, so it sells no model calls');
		const memberID = await requestingMember(request, client, companyID);
		const { data: refusal, error } = await client.rpc('company_member_credit_refusal', {
			target_company: companyID,
			target_member: memberID,
			default_usage_limit: defaultCreditUsageLimit
		});
		if (error) refuse(502, `the record could not say whether this call may start: ${error.message}`);
		if (refusal === 'not_a_member') refuse(403, 'that member is not one of this company');
		if (typeof refusal === 'string') refuse(402, refusal);

		const asked = await askedObject(request);
		const upstream = await fetch(upstreamURL, {
			method: 'POST',
			headers: { ...authorization, 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...asked, usage: { include: true } })
		});
		if (!upstream.ok || !upstream.body) {
			return new Response(upstream.body, { status: upstream.status, headers: { 'Content-Type': 'application/json' } });
		}

		const charge: Charge = { client, companyID, memberID, dollarsPerCredit };
		if (asked.stream === true) return streamedAndCharged(upstream.body, charge);

		const answer: unknown = await upstream.json();
		await charged(charge, costOfAnswer(answer));
		return new Response(JSON.stringify(answer), { headers: { 'Content-Type': 'application/json' } });
	})
);

async function requestingMember(request: Request, client: SupabaseClient, companyID: string): Promise<string | null> {
	const email = (request.headers.get(requesterEmailHeader) ?? '').trim().toLowerCase();
	if (email === '') return null;
	const members = await membersOfCompanyByEmail(client, companyID).catch((failure: unknown) =>
		refuse(502, `the record could not say who asked for this call: ${failure instanceof Error ? failure.message : String(failure)}`)
	);
	const memberID = members.get(email) ?? null;
	if (memberID === null) console.warn(JSON.stringify({ event: 'model_gateway.requester_not_a_member', companyID }));
	return memberID;
}

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
		used_credits: creditsOfCost(cost, charge.dollarsPerCredit)
	});
	if (error) {
		console.error(
			JSON.stringify({ event: 'model_gateway.charge_failed', companyID: charge.companyID, memberID: charge.memberID, cost, reason: error.message })
		);
	}
}
