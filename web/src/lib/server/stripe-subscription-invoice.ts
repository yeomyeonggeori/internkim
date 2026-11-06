import { z } from 'zod';
import type { UnitAmountOfPrice } from './stripe-event-contract';

export const subscriptionCreditsPerMember = 10_000;

export const companyMetadata = z.object({ company_id: z.guid() });

export const paidInvoice = z.object({
	customer: z.string(),
	billing_reason: z.string().nullable(),
	parent: z.object({
		subscription_details: z.object({ subscription: z.string(), metadata: companyMetadata })
	}),
	lines: z.object({
		data: z.array(
			z.object({
				amount: z.number().int(),
				period: z.object({ end: z.number().int() }),
				pricing: z
					.object({ price_details: z.object({ price: z.string() }), unit_amount_decimal: z.string().nullable() })
					.nullable()
			})
		).min(1)
	})
});

type PaidLine = { amount: number; pricing: { price_details: { price: string }; unit_amount_decimal: string | null } | null };

export async function creditsPaidFor(lines: PaidLine[], unitAmountOfPrice: UnitAmountOfPrice): Promise<number> {
	let seatsPaidFor = 0;
	for (const line of lines) {
		if (!line.pricing) continue;
		const unitAmount = line.pricing.unit_amount_decimal === null
			? await unitAmountOfPrice(line.pricing.price_details.price)
			: Number(line.pricing.unit_amount_decimal);
		if (unitAmount > 0) seatsPaidFor += line.amount / unitAmount;
	}
	return Math.max(0, Math.round(subscriptionCreditsPerMember * seatsPaidFor));
}
