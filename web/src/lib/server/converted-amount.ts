import type { ConvertedAmount } from '$lib/currency/converted-amount';
import type { Money } from '$lib/money';
import type { ExchangeRateProvider } from '$lib/server/exchange-rates';
import { minorOfMoney, moneyFromMinor } from './stored-money';
import { convertMinorAmount, minorUnitDigitsOf } from './exchange-rates';

export async function convertedAmount(provider: ExchangeRateProvider, money: Money, to: string): Promise<ConvertedAmount> {
	const rate = await provider.latestRate(money.currency, to);
	const amountMinor = convertMinorAmount(minorOfMoney(money), rate.rate, minorUnitDigitsOf(money.currency), minorUnitDigitsOf(to));
	return { amount: moneyFromMinor(amountMinor, to), rate: rate.rate, asOf: rate.asOf };
}
