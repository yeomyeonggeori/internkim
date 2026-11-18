import type { CompanyCredits } from '$lib/company/credits';
import { isSubscriptionPaying } from '$lib/company/credits-contract';
import { fetchBoxes } from './setup/host-setup-client';

function isTheKeyPayingDuringASubscription(credits: CompanyCredits): boolean {
	return credits.paidBy === 'company_key' && isSubscriptionPaying(credits.subscriptionStatus);
}

export async function payerChangeWaitingOf(credits: CompanyCredits): Promise<boolean | null> {
	if (!isTheKeyPayingDuringASubscription(credits)) return null;
	try {
		return (await fetchBoxes()).isPayerChangeWaiting;
	} catch (failure) {
		console.warn('whether the company computer waits to change what pays for AI could not be read', failure);
		return null;
	}
}
