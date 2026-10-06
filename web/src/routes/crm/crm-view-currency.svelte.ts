import { loadConvertedAmount } from '$lib/currency/converted-amount';
import { isSupabaseConfigured } from '$lib/supabase-session';
import { crmFixtureMode, loadFixtureConvertedAmount } from './dev-crm-fixture';
import { rateHintAnchorCurrency } from './crm-money';

export type CRMViewAmount = {
	value: number;
	currency: string;
	isConverted: boolean;
};

export type CRMViewCurrencyReader = {
	readonly selected: string;
	viewAmount(value: number, currency: string): CRMViewAmount;
};

const ratePreviewAmountMinor = 1000000;

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
				const converted = await this.loadRate(ratePreviewAmountMinor, source, view);
				if (!converted || converted.currencyCode !== view || !Number.isFinite(converted.rate) || converted.rate <= 0) {
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

	viewAmount(value: number, currency: string): CRMViewAmount {
		if (this.selected === '' || currency === this.selected) return { value, currency, isConverted: false };
		const rate = this.ratesBySource[currency];
		if (rate === undefined) return { value, currency, isConverted: false };
		return { value: value * rate, currency: this.selected, isConverted: true };
	}
}

export function isViewCurrencyAvailable(): boolean {
	return isSupabaseConfigured() || crmFixtureMode;
}

export const crmViewCurrency = new CRMViewCurrency(crmFixtureMode ? loadFixtureConvertedAmount : loadConvertedAmount);
