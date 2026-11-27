import { getContext, setContext } from 'svelte';
import { companyCredits } from '$lib/company/credits';
import { readStoredMyCredits, storeMyCredits, type MyCreditUsage } from '$lib/company/my-credits-cache';

const myCreditsKey = Symbol('my-credits');

export class MyCredits {
	credits = $state<MyCreditUsage | null>(null);
	isLoaded = $state(false);
	paysBySubscription = $derived(this.credits?.paidBy === 'subscription');
	private storageScope = '';

	showStored = (scope: string): void => {
		this.storageScope = scope;
		const stored = readStoredMyCredits(scope);
		if (!stored) return;
		this.credits = stored;
		this.isLoaded = true;
	};

	load = async (): Promise<void> => {
		const loaded = await companyCredits().catch(() => null);
		if (loaded) {
			this.credits = loaded;
			storeMyCredits(this.storageScope, loaded);
		}
		this.isLoaded = true;
	};
}

export function provideMyCredits(): MyCredits {
	return setContext(myCreditsKey, new MyCredits());
}

export function myCredits(): MyCredits {
	return getContext<MyCredits>(myCreditsKey);
}
