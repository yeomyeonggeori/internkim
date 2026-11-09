import { z } from 'zod';
import { invokeTool, memberAccessToken } from '$lib/public-api-call';
import { saveCompanySettings } from './company-settings';
import { companyCreditsResultSchema } from './credits-contract';

export type CompanyCredits = z.infer<typeof companyCreditsResultSchema>;

export type CreditTopUpSetting = { enabled: boolean; thresholdCredits: number; amount: number };

const topUpAnswer = z.object({
	setting: z.object({ enabled: z.boolean(), thresholdCredits: z.number(), amount: z.number() }).nullable()
});

const checkoutAnswer = z.object({ url: z.url() });

export async function companyCredits(): Promise<CompanyCredits> {
	return companyCreditsResultSchema.parse(await invokeTool<unknown>('company_credits_get', {}));
}

export async function saveCreditUsageLimit(creditUsageLimit: number): Promise<void> {
	await saveCompanySettings({ creditUsageLimit });
}

async function askBilling(path: string, init: RequestInit = {}): Promise<unknown> {
	const response = await fetch(path, {
		...init,
		headers: { Authorization: `Bearer ${await memberAccessToken()}`, 'Content-Type': 'application/json' }
	});
	const body: unknown = await response.json().catch(() => null);
	if (!response.ok) {
		const message = z.object({ message: z.string() }).safeParse(body);
		throw new Error(message.success ? message.data.message : `${path} answered ${response.status}`);
	}
	return body;
}

export async function creditTopUpSetting(): Promise<CreditTopUpSetting | null> {
	return topUpAnswer.parse(await askBilling('/api/stripe/top-up/settings')).setting;
}

export async function saveCreditTopUpSetting(setting: CreditTopUpSetting): Promise<void> {
	await askBilling('/api/stripe/top-up/settings', { method: 'PUT', body: JSON.stringify(setting) });
}

export async function subscriptionCheckoutURL(): Promise<string> {
	return checkoutAnswer.parse(await askBilling('/api/stripe/checkout', { method: 'POST' })).url;
}
