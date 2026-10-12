import type { ConvertedAmount } from '$lib/currency/converted-amount';
import { roundedMoney, type Money } from '$lib/money';
import type { ExchangeRateProvider } from '$lib/server/exchange-rates';

export async function convertedAmount(provider: ExchangeRateProvider, money: Money, to: string): Promise<ConvertedAmount> {
	const rate = await provider.latestRate(money.currency, to);
	return { amount: roundedMoney(money.amount * rate.rate, to), rate: rate.rate, asOf: rate.asOf };
}
