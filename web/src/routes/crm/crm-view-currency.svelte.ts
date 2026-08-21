import { loadConvertedAmount } from '$lib/currency/converted-amount';
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

export class CRMViewCurrency implements CRMViewCurrencyReader {
	selected = $state<string>('');
	ratesBySource = $state<Record<string, number>>({});
	isLoading = $state(false);
	hasManualChoice = $state(false);

	private lastFailedFollowKey = '';

	constructor(private readonly loadRate: typeof loadConvertedAmount = loadConvertedAmount) {}

	async choose(view: string, sourceCurrencies: string[]): Promise<boolean> {
		this.hasManualChoice = true;
		return this.apply(view, sourceCurrencies);
	}

	async follow(baseCurrency: string, sourceCurrencies: string[]): Promise<void> {
		if (this.isLoading) return;
		const view = this.hasManualChoice ? this.selected : baseCurrency;
		if (!view) return;
		if (this.selected === view && this.hasRatesFor(sourceCurrencies)) return;
		const followKey = `${view}|${[...new Set(sourceCurrencies)].sort().join(',')}`;
		if (followKey === this.lastFailedFollowKey) return;
		const succeeded = await this.apply(view, sourceCurrencies);
		this.lastFailedFollowKey = succeeded ? '' : followKey;
	}

	private hasRatesFor(sourceCurrencies: string[]): boolean {
		return [...sourceCurrencies, rateHintAnchorCurrency].every(
			(source) => source === this.selected || this.ratesBySource[source] !== undefined
		);
	}

	private async apply(view: string, sourceCurrencies: string[]): Promise<boolean> {
		if (!view) return false;
		this.isLoading = true;
		try {
			const rates: Record<string, number> = {};
			for (const source of new Set([...sourceCurrencies, rateHintAnchorCurrency])) {
				if (source === view) continue;
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
