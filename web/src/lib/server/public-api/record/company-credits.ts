import { z } from 'zod';
import { keyringOf } from '../../box';
import { subscriptionCreditsPerMember } from '../../stripe-subscription-invoice';
import { subscriptionPaysForModels } from '../../subscription-pays-for-models';
import { isAModelKeyHeld, payerTheShippedContractCanSay, type CompanyModelPayer, type companyCreditsResultSchema } from '$lib/company/credits-contract';
import type { RecordContext } from './company';
import { creditUsageLimitOf } from './credit-usage-limit';
import { refusalOfPostgresError } from '../refusal';

type CompanyCreditsResult = z.infer<typeof companyCreditsResultSchema>;

const creditColumns = z.object({
	subscription_status: z.string().nullable(),
	subscription_ends_at: z.string().nullable(),
	subscription_credits: z.number(),
	permanent_credits: z.number(),
	rules: z.record(z.string(), z.unknown()).nullable(),
	uses_own_model_key: z.boolean(),
	host_holds_model_key: z.boolean().nullable()
});

const usageRows = z.array(z.object({ member_id: z.string(), credits_used: z.number(), takes_a_seat: z.boolean() }));

async function creditsOfTheCompany(context: RecordContext): Promise<z.infer<typeof creditColumns>> {
	const { data, error } = await context.caller
		.from('company')
		.select('subscription_status, subscription_ends_at, subscription_credits, permanent_credits, rules, uses_own_model_key, host_holds_model_key')
		.eq('id', context.companyID)
		.single();
	if (error) throw refusalOfPostgresError({ code: error.code, message: `reading the credits of company ${context.companyID}: ${error.message}` });
	return creditColumns.parse(data);
}

async function usageTheRequesterMaySee(context: RecordContext): Promise<z.infer<typeof usageRows>> {
	const { data, error } = await context.caller.rpc('company_credit_usage');
	if (error) throw refusalOfPostgresError({ code: error.code, message: `reading the credit usage of company ${context.companyID}: ${error.message}` });
	return usageRows.parse(data ?? []);
}

async function whoPaysForTheModels(context: RecordContext, company: z.infer<typeof creditColumns>): Promise<CompanyModelPayer> {
	const [creditsCanPay, keyring] = await Promise.all([
		subscriptionPaysForModels(context.caller, context.companyID),
		keyringOf(context.accountDirectory, context.companyID)
	]);
	const holdsKey = isAModelKeyHeld(Boolean(keyring?.sealedModelKey), company.host_holds_model_key);
	return payerTheShippedContractCanSay({ holdsKey, usesOwnKey: company.uses_own_model_key, creditsCanPay });
}

export async function companyCreditsGet(context: RecordContext): Promise<CompanyCreditsResult> {
	const [company, usage] = await Promise.all([creditsOfTheCompany(context), usageTheRequesterMaySee(context)]);
	const paidBy = await whoPaysForTheModels(context, company);
	const isAdmin = context.people.some((person) => person.personID === context.requesterID && person.isAdmin);
	const nameOf = new Map(context.people.map((person) => [person.personID, person.name]));
	const seated = usage.filter((row) => row.takes_a_seat);
	return {
		paidBy,
		subscriptionStatus: company.subscription_status,
		periodEndsAt: company.subscription_ends_at,
		billedSeats: isAdmin ? seated.length : null,
		creditsPerSeat: subscriptionCreditsPerMember,
		subscriptionCredits: company.subscription_credits,
		permanentCredits: company.permanent_credits,
		usageLimit: creditUsageLimitOf(company.rules),
		myCreditsUsed: usage.find((row) => row.member_id === context.requesterID)?.credits_used ?? 0,
		members: isAdmin
			? seated.map((row) => ({ personID: row.member_id, name: nameOf.get(row.member_id) ?? '', creditsUsed: row.credits_used }))
			: []
	};
}
