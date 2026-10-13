import { loadConvertedAmount } from '$lib/currency/converted-amount';
import { isSupabaseConfigured } from '$lib/supabase-session';
import { crmFixtureMode, loadFixtureConvertedAmount } from './dev-crm-fixture';
import { rateHintAnchorCurrency } from './crm-money';
import type { Money } from '$lib/money';

export type CRMViewAmount = Money & { isConverted: boolean };

export type CRMViewCurrencyReader = {
	readonly selected: string;
	viewAmount(money: Money): CRMViewAmount;
};

const ratePreviewAmount = 1000000;

function currencyRequestKey(view: string, sourceCurrencies: string[]): string {
	return `${view}|${[...new Set(sourceCurrencies)].sort().join(',')}`;
}

export class CRMViewCurrency implements CRMViewCurrencyReader {
	selected = $state<string>('');
	ratesBySource = $state<Record<string, number>>({});
	isLoading = $state(false);
	hasManualChoice = $state(false);
	pending = $state('');
	failed = $state('');

	private lastFailedFollowKey = '';
	private manualView = '';
	private requestSequence = 0;

	constructor(private readonly loadRate: typeof loadConvertedAmount = loadConvertedAmount) {}

	async choose(view: string, sourceCurrencies: string[]): Promise<boolean> {
		if (!view) return false;
		this.hasManualChoice = true;
		this.manualView = view;
		return this.apply(view, sourceCurrencies);
	}

	async follow(baseCurrency: string, sourceCurrencies: string[]): Promise<void> {
		if (this.isLoading) return;
		const view = this.hasManualChoice ? this.manualView : baseCurrency;
		if (!view) return;
		if (this.selected === view && this.hasRatesFor(sourceCurrencies)) {
			this.failed = '';
			this.lastFailedFollowKey = '';
			return;
		}
		const followKey = currencyRequestKey(view, sourceCurrencies);
		if (followKey === this.lastFailedFollowKey) return;
		await this.apply(view, sourceCurrencies);
	}

	private hasRatesFor(sourceCurrencies: string[]): boolean {
		return [...sourceCurrencies, rateHintAnchorCurrency].every(
			(source) => source === this.selected || this.ratesBySource[source] !== undefined
		);
	}

	private async apply(view: string, sourceCurrencies: string[]): Promise<boolean> {
		if (!view) return false;
		const request = ++this.requestSequence;
		const followKey = currencyRequestKey(view, sourceCurrencies);
		const previousRates = this.selected === view ? this.ratesBySource : {};
		this.isLoading = true;
		this.pending = view;
		this.failed = '';
		try {
			const rates: Record<string, number> = {};
			await Promise.all([...new Set([...sourceCurrencies, rateHintAnchorCurrency])].map(async (source) => {
				if (source === view) return;
				if (previousRates[source] !== undefined) {
					rates[source] = previousRates[source];
					return;
				}
				const converted = await this.loadRate({ amount: ratePreviewAmount, currency: source }, view);
				if (!converted || converted.amount.currency !== view || !Number.isFinite(converted.rate) || converted.rate <= 0) {
					throw new Error('The requested view currency rate is unavailable');
				}
				rates[source] = converted.rate;
			}));
			if (request !== this.requestSequence) return false;
			this.selected = view;
			this.ratesBySource = rates;
			this.lastFailedFollowKey = '';
			return true;
		} catch {
			if (request === this.requestSequence) {
				this.failed = view;
				this.lastFailedFollowKey = followKey;
			}
			return false;
		} finally {
			if (request === this.requestSequence) {
				this.isLoading = false;
				this.pending = '';
			}
		}
	}

	viewAmount(money: Money): CRMViewAmount {
		if (this.selected === '' || money.currency === this.selected) return { ...money, isConverted: false };
		const rate = this.ratesBySource[money.currency];
		if (rate === undefined) return { ...money, isConverted: false };
		return { amount: money.amount * rate, currency: this.selected, isConverted: true };
	}
}

export function isViewCurrencyAvailable(): boolean {
	return isSupabaseConfigured() || crmFixtureMode;
}

export const crmViewCurrency = new CRMViewCurrency(crmFixtureMode ? loadFixtureConvertedAmount : loadConvertedAmount);
