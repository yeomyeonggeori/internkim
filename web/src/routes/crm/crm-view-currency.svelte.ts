import { loadConvertedAmount } from '$lib/currency/converted-amount';

export type CRMViewAmount = {
	value: number;
	currency: string;
	isConverted: boolean;
};

export type CRMViewCurrencyReader = {
	readonly selected: string;
	viewAmount(value: number, currency: string): CRMViewAmount;
};

export const originalViewCurrency = 'original';

const ratePreviewAmountMinor = 1000000;
const sameCurrencyRate = 1;

export class CRMViewCurrency implements CRMViewCurrencyReader {
	selected = $state<string>(originalViewCurrency);
	ratesBySource = $state<Record<string, number>>({});
	isLoading = $state(false);

	constructor(private readonly loadRate: typeof loadConvertedAmount = loadConvertedAmount) {}

	async choose(view: string, sourceCurrencies: string[]): Promise<boolean> {
		if (view === originalViewCurrency) {
			this.selected = originalViewCurrency;
			this.ratesBySource = {};
			return true;
		}
		this.isLoading = true;
		try {
			const rates: Record<string, number> = {};
			for (const source of new Set(sourceCurrencies)) {
				if (source === view) {
					rates[source] = sameCurrencyRate;
					continue;
				}
				const converted = await this.loadRate(ratePreviewAmountMinor, source, view);
				if (!converted) {
					this.selected = originalViewCurrency;
					this.ratesBySource = {};
					return false;
				}
				rates[source] = converted.rate;
			}
			this.selected = view;
			this.ratesBySource = rates;
			return true;
		} finally {
			this.isLoading = false;
		}
	}

	viewAmount(value: number, currency: string): CRMViewAmount {
		if (this.selected === originalViewCurrency) return { value, currency, isConverted: false };
		const rate = this.ratesBySource[currency];
		if (rate === undefined) return { value, currency, isConverted: false };
		return { value: value * rate, currency: this.selected, isConverted: true };
	}
}

export const crmViewCurrency = new CRMViewCurrency();
