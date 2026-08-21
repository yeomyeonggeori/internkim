import { env } from '$env/dynamic/private';
import { asMember } from '$lib/server/control-plane';
import { convertedAmount, type ConvertedAmount } from '$lib/server/converted-amount';
import { frankfurterProvider, type ExchangeRateProvider } from '$lib/server/exchange-rates';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

type CloseRequest = {
	opportunityID: string;
	stage: string;
	stagePosition: number;
	occurredAt: string;
	lostReason: string;
};

type OpportunityAmount = {
	amount_minor: number | null;
	currency_code: string | null;
	base_amount_minor: number | null;
	company_id: string;
};

const provider: ExchangeRateProvider = frankfurterProvider();

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	if (!projectURL || !publishableKey) error(500, 'the central plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const payload = closeRequestOf(await request.json().catch(() => null));
	const client = asMember({ projectURL, publishableKey }, accessToken);

	const opportunity = await client
		.from('opportunity')
		.select('amount_minor, currency_code, base_amount_minor, company_id')
		.eq('id', payload.opportunityID)
		.single<OpportunityAmount>();
	if (opportunity.error) error(404, opportunity.error.message);

	const company = await client
		.from('company')
		.select('currency_code')
		.eq('id', opportunity.data.company_id)
		.single<{ currency_code: string }>();
	if (company.error) error(500, company.error.message);

	const converted = await convertedAmountOf(opportunity.data, company.data.currency_code);

	const settled = await client.rpc('close_crm_opportunity', {
		target_opportunity_id: payload.opportunityID,
		target_stage_id: payload.stage,
		target_stage_position: payload.stagePosition,
		target_stage_changed_at: payload.occurredAt,
		target_lost_reason: payload.lostReason || null,
		target_base_amount_minor: converted?.amountMinor ?? null,
		target_base_currency_code: converted?.currencyCode ?? null
	});
	if (settled.error) error(statusForPostgres(settled.error.code), settled.error.message);

	return json({ opportunity: settled.data, rateAsOf: converted?.asOf ?? null });
};

async function convertedAmountOf(
	opportunity: OpportunityAmount,
	companyCurrency: string
): Promise<ConvertedAmount | null> {
	if (opportunity.base_amount_minor !== null) return null;
	if (opportunity.amount_minor === null || opportunity.currency_code === null) return null;
	if (opportunity.currency_code === companyCurrency) return null;

	return convertedAmount(provider, opportunity.amount_minor, opportunity.currency_code, companyCurrency);
}

function closeRequestOf(value: unknown): CloseRequest {
	if (typeof value !== 'object' || value === null) error(400, 'the close request must be an object');
	const record = value as Record<string, unknown>;
	return {
		opportunityID: requiredString(record, 'opportunityID'),
		stage: requiredString(record, 'stage'),
		stagePosition: requiredNumber(record, 'stagePosition'),
		occurredAt: requiredString(record, 'occurredAt'),
		lostReason: optionalString(record, 'lostReason')
	};
}

function requiredString(record: Record<string, unknown>, key: string): string {
	const value = record[key];
	if (typeof value !== 'string' || value.trim() === '') error(400, `${key} is required`);
	return value;
}

function optionalString(record: Record<string, unknown>, key: string): string {
	const value = record[key];
	if (value === undefined || value === null) return '';
	if (typeof value !== 'string') error(400, `${key} must be a string`);
	return value;
}

function requiredNumber(record: Record<string, unknown>, key: string): number {
	const value = record[key];
	if (typeof value !== 'number' || !Number.isFinite(value)) error(400, `${key} must be a number`);
	return value;
}

function statusForPostgres(code: string | undefined): number {
	if (code === '42501') return 403;
	if (code === '22023') return 422;
	if (code === 'P0002') return 404;
	return 500;
}
