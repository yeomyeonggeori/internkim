import { getContext, setContext } from 'svelte';
import { companyCredits, type CompanyCredits } from '$lib/company/credits';

const myCreditsKey = Symbol('my-credits');

export class MyCredits {
	credits = $state<CompanyCredits | null>(null);
	isLoaded = $state(false);
	paysBySubscription = $derived(this.credits?.paidBy === 'subscription');

	load = async () => {
		this.credits = await companyCredits().catch(() => null);
		this.isLoaded = true;
	};
}

export function provideMyCredits(): MyCredits {
	return setContext(myCreditsKey, new MyCredits());
}

export function myCredits(): MyCredits {
	return getContext<MyCredits>(myCreditsKey);
}
