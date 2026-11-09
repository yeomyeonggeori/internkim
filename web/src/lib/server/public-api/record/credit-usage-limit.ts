import { z } from 'zod';
import { defaultCreditUsageLimit } from '$lib/credit-policy';

const creditPolicy = z.object({ usageLimit: z.number().int().positive() });

export function creditUsageLimitOf(rules: Record<string, unknown> | null): number {
	const policy = creditPolicy.safeParse(rules?.creditPolicy);
	return policy.success ? policy.data.usageLimit : defaultCreditUsageLimit;
}
