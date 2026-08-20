import { convertMinorAmount, minorUnitDigitsOf, type ExchangeRateProvider } from '$lib/server/exchange-rates';

export type ConvertedAmount = { amountMinor: number; currencyCode: string; rate: number; asOf: string };

export async function convertedAmount(
	provider: ExchangeRateProvider,
	amountMinor: number,
	from: string,
	to: string
): Promise<ConvertedAmount> {
	const rate = await provider.latestRate(from, to);
	return {
		amountMinor: convertMinorAmount(amountMinor, rate.rate, minorUnitDigitsOf(from), minorUnitDigitsOf(to)),
		currencyCode: to,
		rate: rate.rate,
		asOf: rate.asOf
	};
}
