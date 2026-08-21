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

const ratePreviewAmountMinor = 1000000;

export class CRMViewCurrency implements CRMViewCurrencyReader {
	selected = $state<string>('');
	ratesBySource = $state<Record<string, number>>({});
	isLoading = $state(false);
	hasManualChoice = $state(false);
	baseCurrency = $state<string>('');

	constructor(private readonly loadRate: typeof loadConvertedAmount = loadConvertedAmount) {}

	async choose(view: string, sourceCurrencies: string[]): Promise<boolean> {
		this.hasManualChoice = true;
		return this.apply(view, sourceCurrencies);
	}

	async follow(baseCurrency: string, sourceCurrencies: string[]): Promise<void> {
		if (this.isLoading) return;
		this.baseCurrency = baseCurrency;
		const view = this.hasManualChoice ? this.selected : baseCurrency;
		if (!view) return;
		if (this.selected === view && this.hasRatesFor(sourceCurrencies)) return;
		await this.apply(view, sourceCurrencies);
	}

	private hasRatesFor(sourceCurrencies: string[]): boolean {
		return [...sourceCurrencies, this.baseCurrency].every(
			(source) => source === '' || source === this.selected || this.ratesBySource[source] !== undefined
		);
	}

	private async apply(view: string, sourceCurrencies: string[]): Promise<boolean> {
		if (!view) return false;
		this.isLoading = true;
		try {
			const rates: Record<string, number> = {};
			for (const source of new Set([...sourceCurrencies, this.baseCurrency])) {
				if (source === '' || source === view) continue;
				const converted = await this.loadRate(ratePreviewAmountMinor, source, view);
				if (!converted) return false;
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
		if (this.selected === '' || currency === this.selected) return { value, currency, isConverted: false };
		const rate = this.ratesBySource[currency];
		if (rate === undefined) return { value, currency, isConverted: false };
		return { value: value * rate, currency: this.selected, isConverted: true };
	}
}

export const crmViewCurrency = new CRMViewCurrency();
